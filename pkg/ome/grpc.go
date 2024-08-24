package ome

import (
	"ccoms/pkg/xetcd"
	"ccoms/pkg/xgrpc"
	"context"
	"encoding/json"
	"strings"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BankServiceClient struct{}

var _ xgrpc.BankServiceClient = (*BankServiceClient)(nil)

func (s *BankServiceClient) Tickets(ctx context.Context, in *xgrpc.ID, opts ...grpc.CallOption) (xgrpc.BankService_TicketsClient, error) {
	return nil, nil
}

func (s *BankServiceClient) BalanceChanges(ctx context.Context, opts ...grpc.CallOption) (xgrpc.BankService_BalanceChangesClient, error) {
	return nil, nil
}

func (w *Worker) PushBalanceChanges(coin string) (err error) {
	grpcUrl, err := xetcd.Get(xetcd.KeyBankService(coin))
	if err != nil {
		return
	}

	grcpClient, err := grpc.Dial(grpcUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}
	defer grcpClient.Close()

	client := xgrpc.NewBankServiceClient(grcpClient)

	chClient, err := client.BalanceChanges(context.Background())
	if err != nil {
		return
	}

	var firstID int64
	err = chClient.Send(&xgrpc.BalanceChange{
		ReasonIDFirst: -1,
		ReasonTable:   "ome_" + strings.ToLower(w.Symbol) + "_logs",
	})
	if err != nil {
		return
	}

	var msg *xgrpc.ID
	for {
		msg, err = chClient.Recv()
		if err != nil {
			return
		}
		firstID = msg.Id

		// push logs
		err = w.PushBalanceLogs(coin, chClient, firstID)
		if err != nil {
			return
		}
	}
}

func (w *Worker) PushBalanceLogs(coin string, chClient xgrpc.BankService_BalanceChangesClient, firstID int64) (err error) {
	ch := make(chan string, 1000)
	ctx := context.Background()

	var push = func(s string) (err error) {
		var bl OmeLog
		err = json.Unmarshal([]byte(s), &bl)
		if err != nil {
			return
		}
		if len(bl.MatchLogs) == 0 {
			return
		}
		ml := bl.MatchLogs[0]

		if bl.LogID <= firstID {
			return
		}

		quantity := IntToDecimal(ml.Quantity)
		amount := IntToDecimal(ml.Amount)

		// BTC TODO distinguish between USDT and BTC
		if coin == w.QuoteAsset {
			// USDT
			err = chClient.Send(&xgrpc.BalanceChange{
				Reason:        "match",
				ReasonTable:   "ome_" + strings.ToLower(w.Symbol) + "_logs",
				ReasonID:      bl.LogID,
				Owner:         ml.Asker,
				FreeChange:    amount.String(),
				FreezeChange:  decimal.Zero.String(),
				Owner2:        ml.Bider,
				FreeChange2:   decimal.Zero.String(),
				FreezeChange2: amount.Neg().String(),
				ReasonIDFirst: firstID,
			})
			if err != nil {
				return
			}
		}
		if coin == w.BaseAsset {
			// BTC
			err = chClient.Send(&xgrpc.BalanceChange{
				Reason:        "match",
				ReasonTable:   "ome_" + strings.ToLower(w.Symbol) + "_logs",
				ReasonID:      bl.LogID,
				Owner:         ml.Asker,
				FreeChange:    decimal.Zero.String(),
				FreezeChange:  quantity.Neg().String(),
				Owner2:        ml.Bider,
				FreeChange2:   quantity.String(),
				FreezeChange2: decimal.Zero.String(),
				ReasonIDFirst: firstID,
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
				err = push(s)
				if err != nil {
					return
				}
			}
		}
	}()

	// TODO starting from the latest id will be more efficient
	err = w.fdb.Tailf(ch)
	if err != nil {
		close(ch)
		return
	}

	return
}

// PullTickets connect to grpc service and continuously receive tickets
func (w *Worker) PullTickets(coin string, ch chan<- *xgrpc.Ticket) (err error) {
	grpcUrl, err := xetcd.Get(xetcd.KeyBankService(coin))
	if err != nil {
		return
	}

	logger.Infof("PullTickets connecting %s", grpcUrl)

	grcpClient, err := grpc.Dial(grpcUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return
	}
	defer func() {
		grcpClient.Close()
		logger.Infof("PullTickets disconnected %s", grpcUrl)
	}()

	logger.Infof("PullTickets connected %s", grpcUrl)

	client := xgrpc.NewBankServiceClient(grcpClient)

	lastID := w.LatestAskTicketID
	if coin == w.QuoteAsset {
		lastID = w.LatestBidTicketID
	}

	chClient, err := client.Tickets(context.Background(), &xgrpc.ID{Id: lastID})
	if err != nil {
		return
	}

	var msg *xgrpc.Ticket
	for {
		msg, err = chClient.Recv()
		if err != nil {
			return
		}
		logger.Tracef("recv new msg(%d) from grpc", msg.Id)
		ch <- msg
	}
}
