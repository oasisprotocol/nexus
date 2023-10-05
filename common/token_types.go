package common

// TokenType is a small-ish number that we use to identify what type of
// token each row of the tokens table is. Values aren't consecutive like in an
// enum. Prefer to use the number of the ERC if applicable and available, e.g.
// 20 for ERC-20.
// "Historical reasons" style note: this has grown to include non-EVM token
// types as well.
type TokenType int

const (
	TokenTypeNative      TokenType = -1 // A placeholder type to represent the runtime's native token. No contract should be assigned this type.
	TokenTypeUnsupported TokenType = 0  // A smart contract for which we're confident it's not a supported token kind.
	TokenTypeERC20       TokenType = 20
	TokenTypeERC721      TokenType = 721
)
