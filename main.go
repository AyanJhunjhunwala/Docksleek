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


func main(){
	dfPath := flag.String("dockerfile", "Dockerfile", "Path to the Dockerfile")

	struct: =
}