package rag

import (
	"fmt"
	"testing"

	"github.com/sjwhitworth/golearn/pca"
	"gonum.org/v1/gonum/mat"
)

func castDownPCA(v []float64, dst int) []float64 {
	d := mat.NewDense(len(v), 1, v)
	p := pca.NewPCA(dst)
	p.Fit(d)
	p.Transform(d)
	rows, cols := p.Transform(d).Dims()
	fmt.Printf("Rows: %d, Cols: %d\n", rows, cols)
	return d.RawMatrix().Data
}

func TestPCA(t *testing.T) {
	r := castDownPCA([]float64{1, 2, 3, 4, 5, 6}, 3)
	fmt.Printf("%+v\n", r)
}
