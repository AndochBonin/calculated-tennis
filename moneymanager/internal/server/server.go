package server

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/order"
	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/risk"
	"github.com/AndochBonin/calculated-tennis/moneymanager/internal/signer"
	moneymanagerv1 "github.com/AndochBonin/calculated-tennis/moneymanager/gen/moneymanager/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Config wires dependencies for the gRPC service.
type Config struct {
	Signer               *signer.Signer
	Allocator            *risk.Allocator
	DefaultDepositWallet string
	DefaultSignatureType uint8
}

// Server implements MoneyManagerService.
type Server struct {
	moneymanagerv1.UnimplementedMoneyManagerServiceServer
	cfg Config
}

func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

func (s *Server) SignOrder(ctx context.Context, req *moneymanagerv1.SignOrderRequest) (*moneymanagerv1.SignOrderResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	side, err := sideFromProto(req.GetSide())
	if err != nil {
		return nil, err
	}
	price, err := parseDecimal(req.GetPrice(), "price")
	if err != nil {
		return nil, err
	}
	size, err := parseDecimal(req.GetSize(), "size")
	if err != nil {
		return nil, err
	}
	if !size.IsPositive() {
		return nil, status.Error(codes.InvalidArgument, "size must be positive")
	}

	tokenID := strings.TrimSpace(req.GetTokenId())
	if tokenID == "" {
		return nil, status.Error(codes.InvalidArgument, "token_id is required")
	}

	depositWallet := strings.TrimSpace(req.GetDepositWallet())
	if depositWallet == "" {
		depositWallet = s.cfg.DefaultDepositWallet
	}
	if depositWallet == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit_wallet is required")
	}
	if !common.IsHexAddress(depositWallet) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid deposit_wallet %q", depositWallet)
	}

	sigType := uint8(req.GetSignatureType())
	if sigType == 0 {
		sigType = s.cfg.DefaultSignatureType
	}

	payload, err := s.cfg.Signer.BuildOrder(
		tokenID,
		side,
		price,
		size,
		req.GetNegRisk(),
		req.GetExpiration(),
		req.GetTimestampMs(),
		depositWallet,
		sigType,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sign order: %v", err)
	}

	return &moneymanagerv1.SignOrderResponse{Order: payloadToProto(payload)}, nil
}

func (s *Server) ProcessSignal(ctx context.Context, req *moneymanagerv1.ProcessSignalRequest) (*moneymanagerv1.ProcessSignalResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if s.cfg.Allocator == nil {
		return nil, status.Error(codes.Internal, "risk allocator not configured")
	}

	side, err := sideFromProto(req.GetSide())
	if err != nil {
		return nil, err
	}
	price, err := parseDecimal(req.GetPrice(), "price")
	if err != nil {
		return nil, err
	}

	tokenID := strings.TrimSpace(req.GetTokenId())
	if tokenID == "" {
		return nil, status.Error(codes.InvalidArgument, "token_id is required")
	}

	depositWallet := s.cfg.DefaultDepositWallet
	if depositWallet == "" {
		return nil, status.Error(codes.InvalidArgument, "deposit wallet not configured on server")
	}
	if !common.IsHexAddress(depositWallet) {
		return nil, status.Errorf(codes.Internal, "invalid default deposit wallet %q", depositWallet)
	}

	if req.WinProbability == nil {
		return nil, status.Error(codes.InvalidArgument, "win_probability is required")
	}
	winProb := req.GetWinProbability()
	riskSide, err := riskSideFromOrder(side)
	if err != nil {
		return nil, err
	}
	if err := risk.ValidatePositiveEV(winProb, riskSide, price); err != nil {
		return nil, err
	}

	size, err := s.cfg.Allocator.Allocate(ctx, risk.Side(side), price)
	if err != nil {
		return nil, err
	}

	payload, err := s.cfg.Signer.BuildOrder(
		tokenID,
		side,
		price,
		size,
		req.GetNegRisk(),
		req.GetExpiration(),
		req.GetTimestampMs(),
		depositWallet,
		s.cfg.DefaultSignatureType,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sign order: %v", err)
	}

	return &moneymanagerv1.ProcessSignalResponse{Order: payloadToProto(payload)}, nil
}

func riskSideFromOrder(s order.Side) (risk.Side, error) {
	switch s {
	case order.SideBuy:
		return risk.SideBuy, nil
	case order.SideSell:
		return risk.SideSell, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "unknown side %q", s)
	}
}

func sideFromProto(s moneymanagerv1.OrderSide) (order.Side, error) {
	switch s {
	case moneymanagerv1.OrderSide_ORDER_SIDE_BUY:
		return order.SideBuy, nil
	case moneymanagerv1.OrderSide_ORDER_SIDE_SELL:
		return order.SideSell, nil
	default:
		return "", status.Error(codes.InvalidArgument, "side is required")
	}
}

func sideToProto(s order.Side) moneymanagerv1.OrderSide {
	switch s {
	case order.SideBuy:
		return moneymanagerv1.OrderSide_ORDER_SIDE_BUY
	case order.SideSell:
		return moneymanagerv1.OrderSide_ORDER_SIDE_SELL
	default:
		return moneymanagerv1.OrderSide_ORDER_SIDE_UNSPECIFIED
	}
}

func parseDecimal(raw, field string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Zero, status.Errorf(codes.InvalidArgument, "%s is required", field)
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, status.Errorf(codes.InvalidArgument, "invalid %s %q: %v", field, raw, err)
	}
	return d, nil
}

func payloadToProto(p *order.Payload) *moneymanagerv1.OrderPayload {
	if p == nil {
		return nil
	}
	return &moneymanagerv1.OrderPayload{
		Maker:         p.Maker,
		Signer:        p.Signer,
		TokenId:       p.TokenID,
		MakerAmount:   p.MakerAmount,
		TakerAmount:   p.TakerAmount,
		Side:          sideToProto(p.Side),
		Expiration:    p.Expiration,
		Timestamp:     p.Timestamp,
		Metadata:      p.Metadata,
		Builder:       p.Builder,
		Signature:     p.Signature,
		Salt:          p.Salt,
		SignatureType: int32(p.SignatureType),
	}
}

// ListenAddrFromEnv returns MONEYMANAGER_GRPC_ADDR (default 127.0.0.1:50051).
func ListenAddrFromEnv() string {
	addr := strings.TrimSpace(os.Getenv("MONEYMANAGER_GRPC_ADDR"))
	if addr == "" {
		return "127.0.0.1:50051"
	}
	return addr
}

// PrivateKeyFromEnv resolves POLYMARKET_PRIVATE_KEY or METAMASK_KEY.
func PrivateKeyFromEnv() (string, error) {
	for _, k := range []string{"POLYMARKET_PRIVATE_KEY", "METAMASK_KEY"} {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("set POLYMARKET_PRIVATE_KEY or METAMASK_KEY")
}
