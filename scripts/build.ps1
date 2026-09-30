# Produces bin\palette.exe, the binary every entrypoint in the manifest runs on
# Windows. herdr clones the repository and runs this during `herdr plugin
# install`, as it runs build.sh on linux and macOS; it has to work under the
# Windows PowerShell 5.1 every Windows ships.
#
# Building from source comes first: the result always matches the checkout. The
# release archive is the fallback for a machine without a Go toolchain, and is
# only used when its checksum matches the checksums.txt published beside it,
# which catches a download cut short or corrupted on the way.
$ErrorActionPreference = 'Stop'
# Windows PowerShell draws a progress bar for a download, which slows it down
# many times over.
$ProgressPreference = 'SilentlyContinue'

Set-Location (Split-Path -Parent $PSScriptRoot)
# Forward slashes, which Windows accepts too: the paths go to go build as they
# are written.
$target = 'bin/palette.exe'
$temp = 'bin/palette.tmp'
New-Item -ItemType Directory -Force -Path bin | Out-Null

function Fail([string]$message) {
	[Console]::Error.WriteLine("build.ps1: $message")
	exit 1
}

# Windows refuses to overwrite an executable that is running, which the palette
# may well be while it is rebuilt, but lets it be renamed. So the old binary is
# moved aside, and what earlier builds moved aside is removed once nothing runs
# it any more.
function Install-Binary {
	Get-ChildItem -Path bin -Filter 'palette.*.old' | Remove-Item -Force -ErrorAction SilentlyContinue
	if (Test-Path $target) {
		Move-Item -Path $target -Destination ("bin/palette.{0}.old" -f [guid]::NewGuid().ToString('N'))
	}
	Move-Item -Path $temp -Destination $target
}

if (Get-Command go -ErrorAction SilentlyContinue) {
	& go build -o $temp ./cmd/palette
	if ($LASTEXITCODE -eq 0) {
		Install-Binary
		exit 0
	}
	Remove-Item -Force -ErrorAction SilentlyContinue $temp
	[Console]::Error.WriteLine('build.ps1: go build failed; falling back to the release binary')
}

$version = Select-String -Path herdr-plugin.toml -Pattern '^version\s*=\s*"([^"]*)"' |
	Select-Object -First 1 |
	ForEach-Object { $_.Matches[0].Groups[1].Value }
if (-not $version) {
	Fail 'herdr-plugin.toml names no version'
}

# A 32-bit PowerShell on a 64-bit Windows reports the machine in
# PROCESSOR_ARCHITEW6432 and itself in PROCESSOR_ARCHITECTURE.
$machine = $env:PROCESSOR_ARCHITEW6432
if (-not $machine) {
	$machine = $env:PROCESSOR_ARCHITECTURE
}
switch ($machine) {
	'AMD64' { $arch = 'amd64' }
	'ARM64' { $arch = 'arm64' }
	default { Fail "no release binary for the processor '$machine'; install Go and retry" }
}

$asset = "palette-windows-$arch.zip"
$release = "https://github.com/vika2603/herdr-palette/releases/download/v$version"

# Windows PowerShell 5.1 on an older .NET offers TLS 1.0 and 1.1 only unless
# told otherwise, and GitHub accepts neither.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

function Get-Release([string]$name, [string]$path) {
	try {
		Invoke-WebRequest -UseBasicParsing -Uri "$release/$name" -OutFile $path
	} catch {
		Fail "could not download $release/${name}: $($_.Exception.Message)"
	}
}

# Unpacked under bin, so the binary is moved into place on one volume.
$tmp = "bin/.download.{0}" -f [guid]::NewGuid().ToString('N')
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
	Get-Release 'checksums.txt' "$tmp/checksums.txt"
	$expected = Get-Content "$tmp/checksums.txt" |
		ForEach-Object { if ($_ -match "^([0-9a-f]{64})  $([regex]::Escape($asset))$") { $Matches[1] } } |
		Select-Object -First 1
	if (-not $expected) {
		Fail "the checksums of v$version have no entry for $asset"
	}

	Get-Release $asset "$tmp/$asset"
	$actual = (Get-FileHash -Algorithm SHA256 -Path "$tmp/$asset").Hash.ToLowerInvariant()
	if ($actual -ne $expected) {
		Fail "checksum mismatch for $asset"
	}

	Expand-Archive -Path "$tmp/$asset" -DestinationPath "$tmp/unpacked"
	Move-Item -Path "$tmp/unpacked/palette.exe" -Destination $temp -Force
	Install-Binary
} finally {
	Remove-Item -Recurse -Force -ErrorAction SilentlyContinue $tmp
}
