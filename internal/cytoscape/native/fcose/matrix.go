// Ported from layout-base 2.0.1 (MIT license): src/util/Matrix.js. Some
// matrix (1d and 2d array) operations.

package fcose

import "math"

// multMat is the matrix multiplication of two 2d arrays.
func multMat(array1, array2 [][]float64) [][]float64 {
	result := make([][]float64, len(array1))

	for i := 0; i < len(array1); i++ {
		result[i] = make([]float64, len(array2[0]))
		for j := 0; j < len(array2[0]); j++ {
			result[i][j] = 0
			for k := 0; k < len(array1[0]); k++ {
				result[i][j] += float64(array1[i][k] * array2[k][j])
			}
		}
	}
	return result
}

// transpose is the transpose of a 2d array.
func transpose(array [][]float64) [][]float64 {
	result := make([][]float64, len(array[0]))

	for i := 0; i < len(array[0]); i++ {
		result[i] = make([]float64, len(array))
		for j := 0; j < len(array); j++ {
			result[i][j] = array[j][i]
		}
	}

	return result
}

// multCons multiplies a 1d array with a constant.
func multCons(array []float64, constant float64) []float64 {
	result := make([]float64, len(array))

	for i := 0; i < len(array); i++ {
		result[i] = array[i] * constant
	}

	return result
}

// minusOp subtracts two 1d arrays.
func minusOp(array1, array2 []float64) []float64 {
	result := make([]float64, len(array1))

	for i := 0; i < len(array1); i++ {
		result[i] = array1[i] - array2[i]
	}

	return result
}

// dotProduct is the dot product of two 1d arrays of the same size.
func dotProduct(array1, array2 []float64) float64 {
	product := 0.0

	for i := 0; i < len(array1); i++ {
		product += float64(array1[i] * array2[i])
	}

	return product
}

// mag is the magnitude of a 1d array.
func mag(array []float64) float64 {
	return math.Sqrt(dotProduct(array, array))
}

// normalize is the normalization of a 1d array.
func normalize(array []float64) []float64 {
	result := make([]float64, len(array))
	magnitude := mag(array)

	for i := 0; i < len(array); i++ {
		result[i] = array[i] / magnitude
	}

	return result
}

// multGamma multiplies a 1d array with the centering matrix.
func multGamma(array []float64) []float64 {
	result := make([]float64, len(array))
	sum := 0.0

	for i := 0; i < len(array); i++ {
		sum += array[i]
	}

	sum = float64(sum * (-1 / float64(len(array))))

	for i := 0; i < len(array); i++ {
		result[i] = sum + array[i]
	}
	return result
}

// multL is a special matrix multiplication: 0.5 * C * INV * C^T * array.
// array and the result are 1d arrays, C and INV are 2d arrays.
func multL(array []float64, c, inv [][]float64) []float64 {
	result := make([]float64, len(c))
	temp1 := make([]float64, len(c[0]))
	temp2 := make([]float64, len(inv))

	// multiply by C^T
	for i := 0; i < len(c[0]); i++ {
		sum := 0.0
		for j := 0; j < len(c); j++ {
			sum += float64(-0.5 * c[j][i] * array[j])
		}
		temp1[i] = sum
	}
	// multiply the result by INV
	for i := 0; i < len(inv); i++ {
		sum := 0.0
		for j := 0; j < len(inv); j++ {
			sum += float64(inv[i][j] * temp1[j])
		}
		temp2[i] = sum
	}
	// multiply the result by C
	for i := 0; i < len(c); i++ {
		sum := 0.0
		for j := 0; j < len(c[0]); j++ {
			sum += float64(c[i][j] * temp2[j])
		}
		result[i] = sum
	}

	return result
}
