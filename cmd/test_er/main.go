package main

import (
	"fmt"
	"github.com/bvolpato/mermaid-go-renderer"
	"io/ioutil"
)

func main() {
	content := `erDiagram
    CUSTOMER ||--o{ ORDER : places
    CUSTOMER {
        string name
        string custNumber
        string sector
    }`

	svg, _ := mermaid.Render(content)
	ioutil.WriteFile("er_test2.svg", []byte(svg), 0644)
	fmt.Println("SVG length:", len(svg))
}
