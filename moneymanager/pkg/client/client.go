// Package client provides a typed gRPC client for the Money Manager service.
package client

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/order"
	moneymanagerv1 "github.com/AndochBonin/calculated-tennis/moneymanager/gen/moneymanager/v1"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

const defaultGRPCAddr = "127.0.0.1:50051"

// Client wraps the generated MoneyManagerService gRPC client.
type Client struct {
	MM   moneymanagerv1.MoneyManagerServiceClient
	conn *grpc.ClientConn
}

// AddrFromEnv returns MONEYMANAGER_GRPC_ADDR (default 127.0.0.1:50051).
func AddrFromEnv() string {
	addr := strings.TrimSpace(os.Getenv("MONEYMANAGER_GRPC_ADDR"))
	if addr == "" {
		return defaultGRPCAddr
	}
	return addr
}

// Dial connects to addr and returns a Client. Call Close when done.
func Dial(ctx context.Context, addr string, opts ...grpc.DialOption) (*Client, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		addr = defaultGRPCAddr
	}
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	dialOpts = append(dialOpts, opts...)
	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("dial money manager %s: %w", addr, err)
	}
	return &Client{
		MM:   moneymanagerv1.NewMoneyManagerServiceClient(conn),
		conn: conn,
	}, nil
}

// DialFromEnv connects using AddrFromEnv.
func DialFromEnv(ctx context.Context, opts ...grpc.DialOption) (*Client, error) {
	return Dial(ctx, AddrFromEnv(), opts...)
}

// NewFromConn returns a Client that does not own conn (no Close side effect on conn).
func NewFromConn(conn *grpc.ClientConn) *Client {
	return &Client{MM: moneymanagerv1.NewMoneyManagerServiceClient(conn)}
}

// Close closes the underlying connection when Dial created it.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// SignLimitOrderParams are the fields sent to SignOrder for an explicit limit order.
type SignLimitOrderParams struct {
	TokenID        string
	Side           order.Side
	Price          decimal.Decimal
	Size           decimal.Decimal
	NegRisk        bool
	Expiration     int64
	TimestampMs    int64
	DepositWallet  string
	SignatureType  uint32
}

// ProcessSignalParams are the fields sent to ProcessSignal for a trade intent.
type ProcessSignalParams struct {
	TokenID         string
	Side            order.Side
	Price           string
	NegRisk         bool
	Expiration      int64
	TimestampMs     int64
	WinProbability  float64 // model P(win), (0,1]; required
}

// ProcessSignal risk-checks a trade intent, allocates size, signs, and returns a payload.
func (c *Client) ProcessSignal(ctx context.Context, p ProcessSignalParams) (*order.Payload, error) {
	if c == nil || c.MM == nil {
		return nil, fmt.Errorf("money manager client is nil")
	}
	winProb := p.WinProbability
	resp, err := c.MM.ProcessSignal(ctx, &moneymanagerv1.ProcessSignalRequest{
		TokenId:        strings.TrimSpace(p.TokenID),
		Side:           SideToProto(p.Side),
		Price:          strings.TrimSpace(p.Price),
		NegRisk:        p.NegRisk,
		Expiration:     p.Expiration,
		TimestampMs:    p.TimestampMs,
		WinProbability: &winProb,
	})
	if err != nil {
		return nil, fmt.Errorf("process signal: %w", err)
	}
	out := PayloadFromProto(resp.GetOrder())
	if out == nil || out.Signature == "" {
		return nil, fmt.Errorf("process signal: empty payload from server")
	}
	return out, nil
}

// SignLimitOrder calls SignOrder and returns a signed order payload.
func (c *Client) SignLimitOrder(ctx context.Context, p SignLimitOrderParams) (*order.Payload, error) {
	if c == nil || c.MM == nil {
		return nil, fmt.Errorf("money manager client is nil")
	}
	resp, err := c.MM.SignOrder(ctx, &moneymanagerv1.SignOrderRequest{
		TokenId:        strings.TrimSpace(p.TokenID),
		Side:           SideToProto(p.Side),
		Price:          p.Price.String(),
		Size:           p.Size.String(),
		NegRisk:        p.NegRisk,
		Expiration:     p.Expiration,
		TimestampMs:    p.TimestampMs,
		DepositWallet:  strings.TrimSpace(p.DepositWallet),
		SignatureType:  p.SignatureType,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Message() != "" {
			return nil, fmt.Errorf("sign order: %s", st.Message())
		}
		return nil, fmt.Errorf("sign order: %w", err)
	}
	out := PayloadFromProto(resp.GetOrder())
	if out == nil || out.Signature == "" {
		return nil, fmt.Errorf("sign order: empty payload from server")
	}
	return out, nil
}
