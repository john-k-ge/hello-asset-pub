package envStructs

type AssetModel struct {
	Id          string `json:"id"`
	Serial      string `json:"serialNo"`
	Description string `json:"description"`
	Uri         string `json:"uri"`
}

type AssetResponse struct {
	Assets []*AssetModel
}
