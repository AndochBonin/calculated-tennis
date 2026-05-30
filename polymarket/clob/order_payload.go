package clob

import (
	"github.com/AndochBonin/E3/moneymanager/pkg/order"
	"github.com/AndochBonin/E3/polymarket/models"
)

func orderSideFromModel(side models.OrderSide) order.Side {
	return order.Side(side)
}

// OrderPayloadFromMoneyManager converts a signed payload from the Money Manager client.
func OrderPayloadFromMoneyManager(p *order.Payload) *models.OrderPayload {
	return orderPayloadToModel(p)
}

func orderPayloadToModel(p *order.Payload) *models.OrderPayload {
	if p == nil {
		return nil
	}
	return &models.OrderPayload{
		Maker:         p.Maker,
		Signer:        p.Signer,
		TokenID:       p.TokenID,
		MakerAmount:   p.MakerAmount,
		TakerAmount:   p.TakerAmount,
		Side:          models.OrderSide(p.Side),
		Expiration:    p.Expiration,
		Timestamp:     p.Timestamp,
		Metadata:      p.Metadata,
		Builder:       p.Builder,
		Signature:     p.Signature,
		Salt:          p.Salt,
		SignatureType: p.SignatureType,
	}
}
