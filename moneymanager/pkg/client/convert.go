package client

import (
	moneymanagerv1 "github.com/AndochBonin/calculated-tennis/moneymanager/gen/moneymanager/v1"
	"github.com/AndochBonin/calculated-tennis/moneymanager/pkg/order"
)

// SideToProto maps a CLOB order side to the protobuf enum.
func SideToProto(s order.Side) moneymanagerv1.OrderSide {
	switch s {
	case order.SideBuy:
		return moneymanagerv1.OrderSide_ORDER_SIDE_BUY
	case order.SideSell:
		return moneymanagerv1.OrderSide_ORDER_SIDE_SELL
	default:
		return moneymanagerv1.OrderSide_ORDER_SIDE_UNSPECIFIED
	}
}

// SideFromProto maps the protobuf enum to a CLOB order side.
func SideFromProto(s moneymanagerv1.OrderSide) order.Side {
	switch s {
	case moneymanagerv1.OrderSide_ORDER_SIDE_BUY:
		return order.SideBuy
	case moneymanagerv1.OrderSide_ORDER_SIDE_SELL:
		return order.SideSell
	default:
		return ""
	}
}

// PayloadFromProto converts a protobuf OrderPayload to the shared order.Payload type.
func PayloadFromProto(p *moneymanagerv1.OrderPayload) *order.Payload {
	if p == nil {
		return nil
	}
	return &order.Payload{
		Maker:         p.GetMaker(),
		Signer:        p.GetSigner(),
		TokenID:       p.GetTokenId(),
		MakerAmount:   p.GetMakerAmount(),
		TakerAmount:   p.GetTakerAmount(),
		Side:          SideFromProto(p.GetSide()),
		Expiration:    p.GetExpiration(),
		Timestamp:     p.GetTimestamp(),
		Metadata:      p.GetMetadata(),
		Builder:       p.GetBuilder(),
		Signature:     p.GetSignature(),
		Salt:          p.GetSalt(),
		SignatureType: int(p.GetSignatureType()),
	}
}
