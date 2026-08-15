$ErrorActionPreference = "Stop"

$out = "coverage.out"
go test ./internal/... -coverprofile=$out -covermode=atomic
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$line = go tool cover -func=$out | Select-String "total:"
if (-not $line) {
  Write-Error "total coverage line not found"
  exit 1
}

# example: total: (statements) 57.3%
if ($line.Line -match "([0-9]+\.[0-9]+)%") {
  $pct = [double]$Matches[1]
} else {
  Write-Error "cannot parse coverage from: $($line.Line)"
  exit 1
}

Write-Host $line.Line
$min = 50.0
if ($pct -lt $min) {
  Write-Error "coverage $pct% is below required $min%"
  exit 1
}

Write-Host "coverage OK (>= $min%)"
