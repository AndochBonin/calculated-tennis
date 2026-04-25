package models

import "github.com/shopspring/decimal"

// Account represents the current state of your trading account.
type Account struct {
	Address      string          
	USDCBalance  decimal.Decimal 
	OpenOrders   []Order         
	Positions    []Position      
	Trades       []Trade         
}
