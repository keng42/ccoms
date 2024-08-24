package bank

import (
	"ccoms/pkg/xetcd"
	"ccoms/pkg/xgrpc"
	"context"
	"encoding/json"
	"io"
	"net"
	"strings"

	"google.golang.org/grpc"
)

type BankServiceServer struct {
	w *Worker
}

var _ xgrpc.BankServiceServer = (*BankServiceServer)(nil)

// BalanceChanges receives balance update requests from ome
func (s *BankServiceServer) BalanceChanges(stream xgrpc.BankService_BalanceChangesServer) (err error) {
	var firstID int64

	for {
		bc, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if bc.ReasonIDFirst == -1 {
			// TODO get firstID from filedb
			firstID = s.w.OmeReasonIDs[bc.ReasonTable]
			err = stream.Send(&xgrpc.ID{Id: firstID})
			if err != nil {
				return err
			}
		}

		if bc.ReasonIDFirst == firstID {
			s.w.ch <- BankMsg{G: bc}
		}
	}
}

// Tickets pushes new tickets to ome
func (s *BankServiceServer) Tickets(id *xgrpc.ID, stream xgrpc.BankService_TicketsServer) (err error) {
	ch := make(chan string, 1024)
	ctx := context.Background()

	var push = func(s string) (err error) {
		var bl BankLog
		err = json.Unmarshal([]byte(s), &bl)
		if err != nil {
			return
		}
		if len(bl.TicketLogs) == 0 {
			return
		}
		for _, tl := range bl.TicketLogs {
			if tl.ID <= id.Id {
				continue
			}
			err = stream.Send(&xgrpc.Ticket{
				Id:       tl.ID,
				Time:     0,
				Owner:    tl.Owner,
				Side:     int64(tl.Side),
				Type:     int64(tl.Type),
				Price:    tl.Price,
				Quantity: tl.Quantity,
			})
			if err != nil {
				return
			}
		}
		return
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case s := <-ch:
				l := len(s)
				if l > 50 {
					l = 50
				}
				logger.Tracef("pushing ticket '%s'", s[0:l])
				err = push(s)
				if err != nil {
					return
				}
			}
		}
	}()

	logger.Infof("tailing filedb")

	// TODO starting from the latest id would be more efficient
	err = s.w.fdb.Tailf(ch)
	if err != nil {
		close(ch)
		return
	}

	return
}

// StartServe starts the grpc service
func (w *Worker) ServeGrpc() (err error) {
	// TODO should retry if etcd get failed
	grpcUrl, err := xetcd.Get(xetcd.KeyBankService(w.Coin))
	if err != nil {
		return
	}

	ss := strings.Split(grpcUrl, ":")
	addr := ":" + ss[1]

	grpcServer := grpc.NewServer()
	srv := &BankServiceServer{w: w}
	xgrpc.RegisterBankServiceServer(grpcServer, srv)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}

	logger.Infof("grpc server listening %s", addr)

	err = grpcServer.Serve(lis)
	if err != nil {
		return
	}

	return
}
