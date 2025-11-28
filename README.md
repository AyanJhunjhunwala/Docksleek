Dockerfile Linter and Image Layer Analyzer

This tool provides Dockerfile linting and optional Docker image layer analysis to help produce smaller, more secure, and more reproducible images.

Features

Dockerfile linting with line-scoped suggestions

Detects common issues: unpinned images, misuse of ADD, inefficient apt usage, missing .dockerignore, supply-chain risks, missing non-root user, missing HEALTHCHECK, and more

Docker image layer analyzer that reports the top three largest layers

Install
git clone https://github.com/<your-org>/<your-repo>.git
cd <your-repo>
go build -o docksleek

Usage
Lint a Dockerfile
./docksleek -dockerfile Dockerfile


Strict mode (exit non-zero if suggestions are found):

./docksleek -dockerfile Dockerfile --strict

Analyze Docker Image Layers

Requires Docker running.

./docksleek -image myimage:latest


Shows the three largest layers with size, timestamps, and creation commands.

Run Both
./docksleek -dockerfile Dockerfile -image myimage:latest

Flags
Flag	Description
-dockerfile <path>	Path to Dockerfile (default: Dockerfile)
-image <name>	Docker image to analyze (optional)
-strict	Exit with non-zero status if any suggestions are found