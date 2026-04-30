package mermaid
import "testing"
import "fmt"
func TestResolveFontPathExported(t *testing.T) {
	fmt.Println("resolve sans-serif:", resolveFontPath("sans-serif"))
	fmt.Println("resolve trebuchet ms:", resolveFontPath("trebuchet ms"))
}
