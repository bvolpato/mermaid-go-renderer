// This file implements original PDF serialization under ISO 32000-1.
// It does not port an upstream implementation.
package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
)

type objectWriter struct {
	objects [][]byte
	err     error
}

func (w *objectWriter) addObject(body string) int {
	w.objects = append(w.objects, []byte(body))
	return len(w.objects)
}

// addStream compresses the data and supplies the stream length.
func (w *objectWriter) addStream(dict string, data []byte) int {
	var packed bytes.Buffer
	z := zlib.NewWriter(&packed)
	if _, err := z.Write(data); err != nil && w.err == nil {
		w.err = err
	}
	if err := z.Close(); err != nil && w.err == nil {
		w.err = err
	}
	var body bytes.Buffer
	fmt.Fprintf(&body, "<< %s /Length %d /Filter /FlateDecode >>\nstream\n", dict, packed.Len())
	body.Write(packed.Bytes())
	body.WriteString("\nendstream")
	w.objects = append(w.objects, body.Bytes())
	return len(w.objects)
}

func (w *objectWriter) bytes(root int) ([]byte, error) {
	if w.err != nil {
		return nil, w.err
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(w.objects)+1)
	for i, body := range w.objects {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.Write(body)
		out.WriteString("\nendobj\n")
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		if int64(offset) > 9999999999 {
			return nil, fmt.Errorf("pdf: object offset exceeds the xref limit")
		}
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), root, xref)
	return out.Bytes(), nil
}
