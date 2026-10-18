# Contributing

Keep the app small and easy to run
Discuss large changes before writing them

## Workflow

Install Go and create a focused branch
Add regression tests for changes in behavior
Use synthetic games and mocked network responses
Run the checks before opening a pull request

```sh
go test -cover ./...
go vet ./...
```

For interface changes also run the browser checks in the README
Explain what changed and how you tested it

## Code

Use standard Go formatting and prefer the standard library
Keep the interface free of build tools
Cache repeated work and bound expensive searches
Do not load models or poll hardware when they are not needed
Measure performance before making speed claims
Keep coaching short and supported by saved positions

## Privacy

Never commit keys or personal directory paths
Keep settings and player data in the ignored data directory
Keep engines and weights outside tracked source
API keys must stay out of files and logs
Profile exports must contain chess data only
Review Git status before committing

## Writing

Use plain labels and short explanations
Avoid decorative punctuation in interface copy and documentation
Preserve required syntax in commands and links

Contributions use GPLv3 or later
Be respectful in issues and reviews
