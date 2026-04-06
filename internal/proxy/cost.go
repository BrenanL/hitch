package proxy

import "github.com/BrenanL/hitch/internal/pricing"

type ModelPricing = pricing.ModelPricing
type Pricing = pricing.Pricing

func PricingFilePath() string              { return pricing.PricingFilePath() }
func LoadPricing() Pricing                 { return pricing.LoadPricing() }
func LoadPricingFromFile(path string) (Pricing, error) {
	return pricing.LoadPricingFromFile(path)
}
func DefaultPricing() Pricing              { return pricing.DefaultPricing() }
func SeedPricingFile() error               { return pricing.SeedPricingFile() }
func WritePricingFile(p Pricing) error     { return pricing.WritePricingFile(p) }
func FetchLiteLLMPricing() (Pricing, error) { return pricing.FetchLiteLLMPricing() }
