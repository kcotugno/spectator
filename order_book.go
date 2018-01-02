package main

import (
	"github.com/emirpasic/gods/trees/redblacktree"
	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"

	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

type Message struct {
	Sequence      int64           `json:"sequence"`
	Type          string          `json:"type"`
	Side          string          `json:"side"`
	Price         decimal.Decimal `json:"price"`
	Size          decimal.Decimal `json:"size"`
	OrderId       string          `json:"order_id"`
	MakerOrderId  string          `json:"maker_order_id"`
	RemainingSize decimal.Decimal `json:"remaining_size"`
	NewSize       decimal.Decimal `json:"new_size"`
	ProductId     string          `json:"product_id"`
	Time          time.Time       `json:"time"`
	Reason        string          `json:"reason"`
	OrderType     string          `json:"order_type"`
	ClientOid     string          `json:"client_oid"`
}

type LevelThree struct {
	Sequence int64             `json:"sequence"`
	Bids     []LevelThreeEntry `json:"bids"`
	Asks     []LevelThreeEntry `json:"asks"`
}

type LevelThreeEntry []string

type Sub struct {
	Type       string   `json:"type"`
	ProductIds []string `json:"product_ids"`
	Channels   []string `json:"channels"`
}

type Entry struct {
	Id    string
	Side  string
	Price decimal.Decimal
	Size  decimal.Decimal
}

type Entries map[string]Entry

type OrderBook struct {
	Msg <-chan Message
	Err <-chan error

	asks *redblacktree.Tree
	bids *redblacktree.Tree

	askLock sync.Mutex
	bidLock sync.Mutex

	msg chan Message
	err chan error

	running  bool
	coin     string
	sequence int64

	conn *websocket.Conn
}

func NewOrderBook(coin string) (*OrderBook, error) {
	var o OrderBook
	var err error

	o.conn, _, err = websocket.DefaultDialer.Dial("wss://ws-feed.gdax.com", nil)
	if err != nil {
		return nil, err
	}

	o.asks = redblacktree.NewWith(DecimalComparator)
	o.bids = redblacktree.NewWith(ReverseDecimalComparator)

	o.msg = make(chan Message, 2048)
	o.Msg = o.msg
	o.err = make(chan error, 0)
	o.Err = o.err

	o.running = true
	o.coin = coin
	o.watchBook()

	return &o, nil
}

func (o *OrderBook) Shutdown() {
	if !o.running {
		return
	}
	o.running = false

	o.conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.
			CloseNormalClosure, ""))
}

func (o *OrderBook) Entries(side string, count int) []Entries {
	entries := make([]Entries, 0)

	tree := o.tree(side)
	lock := o.lock(side)

	lock.Lock()
	defer lock.Unlock()

	it := tree.Iterator()
	for i := 0; i < count; i++ {
		copies := make(Entries, 0)

		ok := it.Next()
		if !ok {
			break
		}

		e := it.Value().(Entries)
		for _, j := range e {
			copies[j.Id] = j
		}

		entries = append(entries, copies)
	}

	return entries

}

func (o *OrderBook) watchBook() {
	go func() {
		defer func() {
			close(o.err)
			close(o.msg)
			o.conn.Close()
			o.running = false
		}()

		var msg Message
		var err error

		sub := Sub{"subscribe", []string{o.coin}, []string{"full"}}

		err = o.conn.WriteJSON(sub)
		if err != nil {
			o.err <- err
			return
		}

		o.loadOrderBook()

		for {
			err := o.conn.ReadJSON(&msg)
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					o.err <- err
				}
				break
			}

			if msg.Sequence <= o.sequence {
				continue
			}

			if msg.Sequence != o.sequence+1 {
				o.loadOrderBook()
				continue
			}

			o.sequence = msg.Sequence

			switch msg.Type {
			case "received":
			case "open":
				o.open(msg)
			case "done":
				o.done(msg)
			case "match":
				o.match(msg)
			case "change":
				o.change(msg)
			default:
				o.err <- errors.New("Unknown message type")
			}

			o.msg <- msg
		}
	}()
}

func (o *OrderBook) open(msg Message) {
	var e Entry

	e.Id = msg.OrderId
	e.Side = msg.Side
	e.Price = msg.Price
	e.Size = msg.RemainingSize

	o.setEntry(e)
}

func (o *OrderBook) done(msg Message) {
	if msg.Price.Equal(decimal.Zero) {
		return
	}

	var e Entry

	e.Id = msg.OrderId
	e.Side = msg.Side
	e.Price = msg.Price

	o.removeEntry(e)
}

func (o *OrderBook) match(msg Message) {
	e, ok := o.entry(msg.Side, msg.Price, msg.MakerOrderId)
	if !ok {
		return
	}

	e.Size = e.Size.Sub(msg.Size)
	o.setEntry(e)

//         if trades.Length() == 256 {
//                 trades.Dequeue()
//         }

//         trades.Enqueue(msg)

//         max := history.Size().Y
//         length := trades.Length()
//         var num int
//         if length > max {
//                 num = max
//         } else {
//                 num = length
//         }

//         for i := 0; i < num; i++ {
//                 var index int

//                 adj := trades.Length() - i - 1

//                 if adj < 0 {
//                         break
//                 } else {
//                         index = adj
//                 }

//                 e := trades.Element(index)

//                 if e != nil {
//                         msg := e.(Message)

//                         var attrs exhibit.Attributes

//                         switch msg.Side {
//                         case "buy":
//                                 attrs.ForegroundColor = exhibit.FGRed
//                         case "sell":
//                                 attrs.ForegroundColor = exhibit.FGGreen
//                         }

//                         le := ListEntry{fmtHistoryEntry(msg), attrs}
//                         history.AddEntry(le)
//                 }
//         }

//         history.Commit()
}

func (o *OrderBook) change(msg Message) {
	var e Entry

	e.Id = msg.OrderId
	e.Side = msg.Side
	e.Price = msg.Price
	e.Size = msg.NewSize

	o.setEntry(e)
}

func (o *OrderBook) loadOrderBook() {
	o.askLock.Lock()
	o.bidLock.Lock()
	o.bids.Clear()
	o.asks.Clear()
	o.askLock.Unlock()
	o.bidLock.Unlock()

	resp, err := http.Get(fmt.Sprintf("https://api.gdax.com/products/%v/book?level=3", coin))
	if err != nil {
		o.err <- err
		o.Shutdown()
		return
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		o.err <- err
		o.Shutdown()
		return
	}

	var parsed LevelThree

	err = json.Unmarshal(buf, &parsed)
	if err != nil {
		o.err <- err
		o.Shutdown()
		return
	}

	for _, b := range parsed.Bids {
		var e Entry

		e.Price, err = decimal.NewFromString(b[0])
		if err != nil {
			o.err <- err
			o.Shutdown()
			return
		}
		e.Size, err = decimal.NewFromString(b[1])
		if err != nil {
			o.err <- err
			o.Shutdown()
			return
		}
		e.Id = b[2]
		e.Side = "buy"

		o.setEntry(e)
	}

	for _, a := range parsed.Asks {
		var e Entry

		e.Price, err = decimal.NewFromString(a[0])
		if err != nil {
			o.err <- err
			o.Shutdown()
			return
		}
		e.Size, err = decimal.NewFromString(a[1])
		if err != nil {
			o.err <- err
			o.Shutdown()
			return
		}
		e.Id = a[2]
		e.Side = "sell"

		o.setEntry(e)
	}

	o.sequence = parsed.Sequence
}

func (o *OrderBook) lock(side string) *sync.Mutex {
	switch side {
	case "sell":
		return &o.askLock
	case "buy":
		return &o.bidLock
	}

	return nil
}

func (o *OrderBook) tree(side string) *redblacktree.Tree {
	switch side {
	case "sell":
		return o.asks
	case "buy":
		return o.bids
	}

	return nil
}

func (o *OrderBook) entries(side string, key decimal.Decimal) (Entries, bool) {
	tree := o.tree(side)
	lock := o.lock(side)

	lock.Lock()
	defer lock.Unlock()

	values, ok := tree.Get(key)

	if !ok {
		return nil, false
	}

	entries := make(Entries)
	for k, v := range values.(Entries) {
		entries[k] = v
	}

	return entries, true
}

func (o *OrderBook) updateEntries(side string, price decimal.Decimal, e Entries) {
	tree := o.tree(side)
	lock := o.lock(side)

	lock.Lock()
	defer lock.Unlock()

	if len(e) == 0 {
		tree.Remove(price)
	} else {
		tree.Put(price, e)
	}
}

func (o *OrderBook) entry(side string, price decimal.Decimal,
	id string) (Entry, bool) {
	var entry Entry

	entries, ok := o.entries(side, price)
	if !ok {
		return entry, false
	}

	entry, ok = entries[id]
	return entry, ok
}

func (o *OrderBook) setEntry(e Entry) {
	entries, ok := o.entries(e.Side, e.Price)
	if !ok {
		entries = Entries{}
	}

	entries[e.Id] = e
	o.updateEntries(e.Side, e.Price, entries)
}

func (o *OrderBook) removeEntry(e Entry) {
	entries, ok := o.entries(e.Side, e.Price)
	if !ok {
		return
	}

	delete(entries, e.Id)
	o.updateEntries(e.Side, e.Price, entries)
}

func DecimalComparator(a, b interface{}) int {
	aAsserted := a.(decimal.Decimal)
	bAsserted := b.(decimal.Decimal)

	switch {
	case aAsserted.GreaterThan(bAsserted):
		return 1
	case aAsserted.LessThan(bAsserted):
		return -1
	default:
		return 0
	}
}

func ReverseDecimalComparator(a, b interface{}) int {
	aAsserted := a.(decimal.Decimal)
	bAsserted := b.(decimal.Decimal)

	switch {
	case aAsserted.GreaterThan(bAsserted):
		return -1
	case aAsserted.LessThan(bAsserted):
		return 1
	default:
		return 0
	}
}
