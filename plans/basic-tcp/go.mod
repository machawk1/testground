module github.com/ipfs/testground/plans/basic-tcp

go 1.13

require (
	github.com/ipfs/testground/sdk/runtime v0.0.0-00010101000000-000000000000
	github.com/ipfs/testground/sdk/sync v0.0.0-20191017072543-376444a0dd33
)

replace (
	github.com/ipfs/testground/sdk/runtime => ../../sdk/runtime
	github.com/ipfs/testground/sdk/sync => ../../sdk/sync
)
