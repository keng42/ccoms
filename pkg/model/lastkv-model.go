package model

// Lastkv model
//
// Used to record some values. For example, the latest seq of nats messages, because not every log is sent through nats,
// some are sent through grpc, so there will be no seq, so it needs to be recorded here separately.
// Similarly, there is also the latest ticketID carried by the ome request.
type Lastkv struct {
	ID int64 `json:"id" gorm:"omitempty; primaryKey;"`

	App string `json:"app" gorm:"omitempty; not null; default:''; type:varchar(64); uniqueindex:idx_app_key;"` // e.g bank_usdt
	Key string `json:"key" gorm:"omitempty; not null; default:''; type:varchar(64); uniqueindex:idx_app_key;"` // e.g nats_seq, ome_reasonid_btc_usdt
	Val int64  `json:"val" gorm:"omitempty; not null; default:0;"`

	Model
}

const (
	LASTKV_K_NATS_SEQ             = "nats_seq"
	LASTKV_K_SAVED_LOG_ID         = "saved_log_id"
	LASTKV_K_LATEST_ORDER_ID      = "latest_order_id"
	LASTKV_K_LATEST_ASK_TICKET_ID = "latest_ask_ticket_id"
	LASTKV_K_LATEST_BID_TICKET_ID = "latest_bid_ticket_id"
	LASTKV_K_OME_REASONID         = "ome_reasonid_" // this+symbol
)
