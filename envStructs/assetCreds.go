package envStructs

type AssetConfig struct {
	HeaderName string `json:"http-header-name"`
	HeaderVal  string `json:"http-header-value"`
	Scope      string `json:"oauth-scope"`
	Url        string
}
