# Contributing

Thanks for your interest in contributing!

## How to Contribute

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Development Setup

The canonical quality gate is BuildFlow, which auto-detects this repo and
runs lint, format, and tests with cache isolation:

    buildflow

Plain Go commands work too (Go 1.27+, no GOEXPERIMENT flags needed):

    go test ./... -race
    golangci-lint run ./...

## Reporting Issues

Please use GitHub Issues to report bugs or request features.
