package main
import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type finding struct{
	line int
	msg string
}

//Starting point
func main(){
	dfPath := flag.String("dockerfile", "Dockerfile", "Path to the Dockerfile")

	strict := flag.Bool("strict", false, "Exit with non-zero status if any suggestions are found (useful in CI)")

	flag.Parse()

	content, err := os.ReadFile(*dfPath)
	if err != nil {
		fmt.Printf("Error reading Dockerfile: %v\n", err)
		return
	}
	findings := lintDockerfile(string(content))

	findings = append(findings, checkDotDockerignore(*dfPath)...)

		if len(findings) == 0 {
		fmt.Println("No suggestions  (Nice Dockerfile!)")
		return
	}

	fmt.Println("Suggestions:")
	for _, f := range findings {
		if f.line > 0 {
			//line-scoped suggestion.
			fmt.Printf("  [L%d] %s\n", f.line, f.msg)
		} else {
			fmt.Printf("  %s\n", f.msg)
		}
	}

	if *strict {
		os.Exit(1)
	}

}