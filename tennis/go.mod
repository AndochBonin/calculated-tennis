module github.com/AndochBonin/E3/tennis

go 1.25.3

require (
	github.com/AndochBonin/E3/moneymanager v0.0.0
	github.com/PuerkitoBio/goquery v1.12.0
	github.com/alicebob/miniredis/v2 v2.38.0
	github.com/ethereum/go-ethereum v1.17.2
	github.com/joho/godotenv v1.5.1
	github.com/redis/go-redis/v9 v9.20.0
	github.com/shopspring/decimal v1.4.0
	golang.org/x/sync v0.20.0
)

replace github.com/AndochBonin/E3/moneymanager => ../moneymanager

require (
	github.com/andybalholm/cascadia v1.3.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/holiman/uint256 v1.3.2 // indirect
	github.com/yuin/gopher-lua v1.1.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
