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


func lintDockerfile(content string) []finding { // Lints Dockerfile content and returns suggestions.
	var out []finding 

	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Split(bufio.ScanLines) // Ensure we split by lines.

	lineNo := 0               
	haveUserNonRoot := false   
	lastStageStartLine := 0    
	healthcheckSeen := false   
	exposeSeen := false        
	cmdLooksLikeServer := false 
	fromCount := 0             

	reFrom := regexp.MustCompile(`(?i)^\s*FROM\s+([^\s:]+)(?::([^\s]+))?`)  // FROM image[:tag]
	reAdd := regexp.MustCompile(`(?i)^\s*ADD\s+`)                           // ADD instruction (prefer COPY).
	reCopyDot := regexp.MustCompile(`(?i)^\s*COPY\s+--?chown=\S+\s+\.\s+\.\s*$|^\s*COPY\s+\.\s+\.\s*$`) // COPY . .
	reAptUpdate := regexp.MustCompile(`(?i)apt(-get)?\s+update`)             // apt-get update
	reAptInstall := regexp.MustCompile(`(?i)apt(-get)?\s+install(\s|$)`)     // apt-get install
	reNoRecommends := regexp.MustCompile(`(?i)--no-install-recommends`)      // APT flag to reduce image bloat.
	reRmAptLists := regexp.MustCompile(`/var/lib/apt/lists/\*`)              // APT cache cleanup path.
	rePipInstall := regexp.MustCompile(`(?i)pip(3)?\s+install(\s|$)`)        // pip install
	reNpmInstall := regexp.MustCompile(`(?i)npm\s+install(\s|$)`)            // npm install (prefer npm ci).
	reCurlPipeSh := regexp.MustCompile(`(?i)curl[^\|]*\|\s*(sh|bash)`)       // curl | sh
	reWgetPipeSh := regexp.MustCompile(`(?i)wget[^\|]*\|\s*(sh|bash)`)       // wget | bash
	reUser := regexp.MustCompile(`(?i)^\s*USER\s+(.+)$`)                     // USER <name|uid>
	reExpose := regexp.MustCompile(`(?i)^\s*EXPOSE\s+`)                      // EXPOSE <port>...
	reHealth := regexp.MustCompile(`(?i)^\s*HEALTHCHECK\s+`)                 // HEALTHCHECK ...
	reCmd := regexp.MustCompile(`(?i)^\s*CMD\s+`)                            // CMD ...

	for sc.Scan() {
		line := sc.Text()         
		lineNo++                 
		withoutComments := stripTrailingComment(line) 

		if m := reFrom.FindStringSubmatch(withoutComments); m != nil {
			fromCount++                     // Count this stage.
			lastStageStartLine = lineNo     // Remember where the final stage starts.
			image := m[1]                   // Captured base image (e.g., ubuntu).
			tag := m[2]                     // Optional tag (may be empty if omitted).
			if strings.EqualFold(tag, "latest") || tag == "" {
				if tag == "" {
					out = append(out, finding{
						lineNo,
						fmt.Sprintf("Stage base image '%s' is unpinned. Pin to a specific tag or digest for reproducible builds (e.g., '%s:1.2.3' or '@sha256:...').", image, image),
					})
				} else {
					out = append(out, finding{
						lineNo,
						fmt.Sprintf("Avoid using ':latest' on base image '%s'. Pin to a specific tag or digest for reproducible builds.", image),
					})
				}
			}
			haveUserNonRoot = false
		}

		if reAdd.MatchString(withoutComments) {
			out = append(out, finding{
				lineNo,
				"Use COPY instead of ADD unless you specifically need tar auto-extraction or remote URL downloads.",
			})
		}

		if reCopyDot.MatchString(withoutComments) {
			out = append(out, finding{
				lineNo,
				"Avoid 'COPY . .'. Copy only needed paths (e.g., 'COPY go.mod go.sum ./', then 'COPY ./cmd/app ./cmd/app') to improve cache hits and reduce image size.",
			})
		}

		if strings.Contains(strings.ToLower(withoutComments), "apt") {
			update := reAptUpdate.MatchString(withoutComments)  
			install := reAptInstall.MatchString(withoutComments) 

			if install && !reNoRecommends.MatchString(withoutComments) {
				out = append(out, finding{
					lineNo,
					"Use 'apt-get install --no-install-recommends' to reduce image size.",
				})
			}

			if (update && !install) || (install && !update) {
				out = append(out, finding{
					lineNo,
					"Combine 'apt-get update' and 'apt-get install' in the same RUN to avoid stale caches (e.g., 'RUN apt-get update && apt-get install -y ...').",
				})
			}

			// Also clean caches in the same layer.
			if (update || install) && !reRmAptLists.MatchString(withoutComments) {
				out = append(out, finding{
					lineNo,
					"Clean apt cache in the same RUN layer (e.g., 'rm -rf /var/lib/apt/lists/*') to keep images slim.",
				})
			}
		}

		// (5) pip best practice: --no-cache-dir prevents caching wheels/packages in the image.
		if rePipInstall.MatchString(withoutComments) && !strings.Contains(strings.ToLower(withoutComments), "--no-cache-dir") {
			out = append(out, finding{
				lineNo,
				"Use 'pip install --no-cache-dir ...' to avoid leaving wheel/cache artifacts in the image.",
			})
		}

		// (6) npm best practice: prefer 'npm ci' with a lockfile for reproducible installs.
		if reNpmInstall.MatchString(withoutComments) && !strings.Contains(strings.ToLower(withoutComments), "ci") {
			out = append(out, finding{
				lineNo,
				"Prefer 'npm ci' (with a lockfile) in CI builds for reproducible installs.",
			})
		}

		// (7) Supply-chain hardening: avoid piping network content into a shell.
		if reCurlPipeSh.MatchString(withoutComments) ||reWgetPipeSh.MatchString(withoutComments) {
			out = append(out, finding{
				lineNo, 
				"Avoid piping curl/wget to shell. Download, verify checksum/signature, then execute. This hardens against supply-chain risks.",
			})
		}

		// (8) Non-root user detection: mark if stage sets USER to something other than root.
		if m := reUser.FindStringSubmatch(withoutComments); m != nil {
			if strings.TrimSpace(strings.ToLower(m[1])) != "root" {
				haveUserNonRoot = true // Final stage passes this check if it ever sets non-root.
			}
		}

		// (9) Healthcheck heuristic: server-like images should have a HEALTHCHECK.
		if reExpose.MatchString(withoutComments) {
			exposeSeen = true 
		}
		if reCmd.MatchString(withoutComments) {
			lc := strings.ToLower(withoutComments)
			if strings.Contains(lc, "nginx") ||
				strings.Contains(lc, "http-server") ||
				strings.Contains(lc, "gunicorn") ||
				strings.Contains(lc, "uvicorn") ||
				strings.Contains(lc, "node") ||
				strings.Contains(lc, "serve") {
				cmdLooksLikeServer = true
			}
		}
		if reHealth.MatchString(withoutComments) {
			healthcheckSeen = true // HEALTHCHECK present somewhere—good.
		}
	}

	// ----- Post-scan checks (after reading the whole Dockerfile) -----

	if !haveUserNonRoot {
		line := 0
		if lastStageStartLine > 0 {
			line = lastStageStartLine
		}
		out = append(out, finding{
			line,
			"No non-root USER detected in the final stage. Run as a non-root user for better security (e.g., 'RUN useradd -m app && USER app').",
		})
	}

	if (exposeSeen || cmdLooksLikeServer) && !healthcheckSeen {
		out = append(out, finding{
			0,
			"Consider adding a HEALTHCHECK to detect when the container is unhealthy (e.g., 'HEALTHCHECK CMD curl -f http://localhost:8080/health || exit 1').",
		})
	}

	if fromCount >= 2 {
		out = append(out, finding{
			0,
			"Multistage build detected. Ensure you only copy the built artifacts from builder stages (e.g., 'COPY --from=builder /app/bin/myapp /usr/local/bin/myapp').",
		})
	}

	return dedupeFindings(out)
}

// stripTrailingComment removes trailing '#' comments that are OUTSIDE quotes.
// It preserves '#' inside single or double quotes (common in shell commands).
func stripTrailingComment(line string) string {
	inSingle := false // Track whether we’re inside single quotes '...'.
	inDouble := false // Track whether we’re inside double quotes "...".
	for i, r := range line { // Walk runes to handle Unicode and quote toggling correctly.
		switch r {
		case '\'':
			if !inDouble { // Only toggle single-quote state if not inside double quotes.
				inSingle = !inSingle
			}
		case '"':
			if !inSingle { // Only toggle double-quote state if not inside single quotes.
				inDouble = !inDouble
			}
		case '#':
			// If we see '#', and we’re not inside any quotes, treat it as a comment start.
			if !inSingle && !inDouble {
				return strings.TrimSpace(line[:i]) // Return everything before the '#'.
			}
		}
	}
	return strings.TrimSpace(line) // No comment found outside quotes; just trim whitespace.
}

func dedupeFindings(f []finding) []finding {
	seen := make(map[string]bool) // Remember keys we've emitted.
	var out []finding             // Output accumulator.
	for _, x := range f {
		key := fmt.Sprintf("%d|%s", x.line, x.msg) // Composite key.
		if !seen[key] {
			seen[key] = true       // Mark as seen.
			out = append(out, x)   // Keep first occurrence.
		}
	}
	return out // Return deduped slice.
}


// checkDotDockerignore inspects whether a .dockerignore exists next to the Dockerfile
// and suggests common ignores if missing.
func checkDotDockerignore(dfPath string) []finding {
	var out []finding // Results accumulator.

	// Resolve absolute path to the Dockerfile for reliable neighbor lookup.
	dfAbs := dfPath
	if !filepath.IsAbs(dfAbs) {
		if abs, err := filepath.Abs(dfAbs); err == nil {
			dfAbs = abs
		}
	}

	buildCtxDir := filepath.Dir(dfAbs)

	ignorePath := filepath.Join(buildCtxDir, ".dockerignore")

	// Try to read .dockerignore; if missing, suggest creating one with sensible defaults.
	data, err := os.ReadFile(ignorePath)
	if err != nil {
		out = append(out, finding{
			0,
			fmt.Sprintf(".dockerignore not found next to the Dockerfile (%s). Add one to reduce context size and speed up builds.", ignorePath),
		})
		out = append(out, finding{
			0,
			"Suggested entries for .dockerignore: .git, .gitignore, node_modules, dist, build, target, .venv, *.log, *.tmp, **/.DS_Store, .env, secrets/*",
		})
		return out // No more checks possible without a file.
	}

	lines := parseDockerignore(string(data))

	recommended := []string{
		".git",
		".gitignore",
		"node_modules",
		"dist",
		"build",
		"target",
		".venv",
		"*.log",
		"*.tmp",
		"**/.DS_Store",
		".env",
		"secrets/*",
	}

	missing := diff(recommended, lines)
	if len(missing) > 0 {
		out = append(out, finding{
			0,
			fmt.Sprintf(".dockerignore exists but is missing common entries: %s", strings.Join(missing, ", ")),
		})
	}

	return out // Return .dockerignore findings.
}

func parseDockerignore(content string) []string {
	var out []string                                 // Collected patterns.
	sc := bufio.NewScanner(strings.NewReader(content)) // Scanner over file content.
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text()) // Trim spaces for robustness.
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip blank lines and comments.
		}
		out = append(out, line) // Keep pattern lines.
	}
	return out // All patterns collected.
}

// diff returns which strings in `need` are NOT present in `have`.
func diff(need, have []string) []string {
	set := make(map[string]bool, len(have)) // Build a membership set for fast lookup.
	for _, h := range have {
		set[h] = true
	}
	var missing []string
	for _, n := range need {
		if !set[n] {
			missing = append(missing, n) // Accumulate missing entries.
		}
	}
	return missing // Return the missing subset.
}
