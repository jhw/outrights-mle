package outrightsmle

import (
	"math"
	"math/rand"
)

// PoissonProb calculates Poisson probability P(X = k) where X ~ Poisson(lambda)
func PoissonProb(lambda float64, k int) float64 {
	if k < 0 {
		return 0
	}
	if lambda <= 0 {
		if k == 0 {
			return 1.0
		}
		return 0
	}
	
	// Use log space for numerical stability
	logProb := float64(k)*math.Log(lambda) - lambda - logFactorial(k)
	return math.Exp(logProb)
}

// PoissonSample generates a random sample from Poisson distribution
func PoissonSample(lambda float64) int {
	if lambda < 0 {
		return 0
	}
	
	// Use inverse transform sampling for small lambda
	if lambda < 12 {
		L := math.Exp(-lambda)
		k := 0
		p := 1.0
		
		for p > L {
			k++
			p *= rand.Float64()
		}
		return k - 1
	}
	
	// Use normal approximation for large lambda
	return int(math.Max(0, rand.NormFloat64()*math.Sqrt(lambda)+lambda+0.5))
}

// logFactorial computes log(n!) for Poisson calculations
func logFactorial(n int) float64 {
	if n <= 1 {
		return 0
	}
	result := 0.0
	for i := 2; i <= n; i++ {
		result += math.Log(float64(i))
	}
	return result
}