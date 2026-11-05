package typebreader

import "testing"

// Whatever the input, Read never panics, and a failure is always a ParseError on a line from 1.
// A message that reads is accepted by the typed reader of its type and refused, with a ParseError
// on a line from 1, by the other one.
func FuzzRead(f *testing.F) {
	f.Add(benchSSM)
	f.Add(benchMVT)
	f.Add("QU MADRMXA\n.MADRMXA 050812\nMVT\nXA123/05.ECABC.MAD\nAD0805 EO0820 EA0925 BCN\nDL64/72/0015/0020\nPX12/134/10\nSI DEICING")
	f.Add("QU MADRMXA\n.MADRMXA 050700\nMVT\nXA123/05.ECABC.MAD\nED051130\nDL72/0025")
	f.Add("")
	f.Add("QU MADRMXA\r\n.MADRMXA 050812\r\nMVT\r\nXA123/05.ECABC.MAD\r\nAD0805/0812\r\n")
	f.Add("QU MADRMXA\n.MADRMXA 050812\nMVT\nXA123/05.ECABC.MAD\nAD0805/0812\rXYZ")
	f.Add(" QU  MADRMXA\t\n.MADRMXA 050812\nMVT")
	f.Add("QU MADRMXA\n.BCNRMXA 051530")
	f.Add("QU MADRMXA\n.BCNRMXA 051530\nSSM\nUTC\nTIM\nXA123\n05NOV26 28MAR27 135\nMAD0830 BCN0945\n")
	r := NewReader()
	f.Fuzz(func(t *testing.T, text string) {
		message, err := r.Read(text)
		if err != nil {
			perr, ok := err.(*ParseError)
			if !ok || perr.Line < 1 {
				t.Fatalf("Read(%q) failed with %#v", text, err)
			}
			return
		}
		// The typed reader of the message's own type accepts it. The other one refuses it, with a line.
		_, ssmErr := ReadSSM(message)
		_, mvtErr := ReadMVT(message)
		own, other := ssmErr, mvtErr
		if message.Type == "MVT" {
			own, other = mvtErr, ssmErr
		}
		if own != nil {
			t.Fatalf("Read(%q) succeeded but the typed reader failed: %v", text, own)
		}
		if perr, ok := other.(*ParseError); !ok || perr.Line < 1 {
			t.Fatalf("Read(%q): the typed reader of the other type failed with %#v", text, other)
		}
	})
}
