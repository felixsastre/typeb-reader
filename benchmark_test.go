package typebreader

import (
	"strings"
	"testing"
)

var (
	benchSSM = strings.Join([]string{
		"QU MADRMXA", ".BCNRMXA 051530", "SSM", "UTC", "NEW", "XA123",
		"05NOV26 28MAR27 135", "J 738 Y180", "MAD0810 BCN0925",
	}, "\n")
	benchMVT = strings.Join([]string{
		"QU MADRMXA", ".MADRMXA 050812", "MVT", "XA123/05.ECABC.MAD", "AD0805/0812",
	}, "\n")
)

func benchmarkRead(b *testing.B, text string) {
	r := NewReader()
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := r.Read(text); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadSSM(b *testing.B) { benchmarkRead(b, benchSSM) }
func BenchmarkReadMVT(b *testing.B) { benchmarkRead(b, benchMVT) }

func BenchmarkNewReader(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		NewReader()
	}
}
