package officerender

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Rasteritzador de PDF integrat a Windows 10/11 (Windows.Data.Pdf),
// cridat des de Windows PowerShell 5.1 (pwsh 7 ja no projecta WinRT).
const winPDFScript = `param([string]$Pdf, [string]$OutDir, [int]$Width = 1280, [string]$Pages = '')
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asOp = @([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation` + "`" + `1' })[0]
$asAct = @([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and -not $_.IsGenericMethod -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncAction' })[0]
function Await($op, [Type]$t) { $task = $asOp.MakeGenericMethod($t).Invoke($null, @($op)); $task.Wait(-1) | Out-Null; $task.Result }
function AwaitAct($a) { $task = $asAct.Invoke($null, @($a)); $task.Wait(-1) | Out-Null }
[void][Windows.Storage.StorageFile, Windows.Storage, ContentType = WindowsRuntime]
[void][Windows.Data.Pdf.PdfDocument, Windows.Data.Pdf, ContentType = WindowsRuntime]
[void][Windows.Storage.Streams.IRandomAccessStream, Windows.Storage.Streams, ContentType = WindowsRuntime]
$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($Pdf)) ([Windows.Storage.StorageFile])
$doc = Await ([Windows.Data.Pdf.PdfDocument]::LoadFromFileAsync($file)) ([Windows.Data.Pdf.PdfDocument])
$folder = Await ([Windows.Storage.StorageFolder]::GetFolderFromPathAsync($OutDir)) ([Windows.Storage.StorageFolder])
$n = [int]$doc.PageCount
Write-Output "PAGES $n"
$list = @()
if ($Pages -ne '') { $list = $Pages.Split(':') | ForEach-Object { [int]$_ } } else { $list = 1..$n }
foreach ($i in $list) {
  if ($i -lt 1 -or $i -gt $n) { continue }
  $page = $doc.GetPage($i - 1)
  $out = Await ($folder.CreateFileAsync("page-$i.png", [Windows.Storage.CreationCollisionOption]::ReplaceExisting)) ([Windows.Storage.StorageFile])
  $stream = Await ($out.OpenAsync([Windows.Storage.FileAccessMode]::ReadWrite)) ([Windows.Storage.Streams.IRandomAccessStream])
  $opts = New-Object Windows.Data.Pdf.PdfPageRenderOptions
  $opts.DestinationWidth = $Width
  AwaitAct ($page.RenderToStreamAsync($stream, $opts))
  $stream.Dispose(); $page.Dispose()
  Write-Output "OK $i"
}
`

// Exportació amb Microsoft Office per COM (quan no hi ha LibreOffice).
const comScript = `param([string]$Src, [string]$OutDir, [string]$Kind, [int]$Width = 1280, [string]$Pages = '')
$ErrorActionPreference = 'Stop'
# Cadenes de debò: el que torna Join-Path arribava a COM embolcallat i Word
# es penjava, Excel deia «mateix nom que un altre llibre» i PowerPoint
# «memòria insuficient» (vist el 2026-10-28 amb Office 16).
$pdf = [string](Join-Path $OutDir 'src.pdf')
$Src = [string]$Src
# El procés d'Office que obrim ve de DCOM, no és fill d'aquest script: si
# Gregal ha de tallar per temps, en necessita el PID. Només el d'una
# instància NOVA: si l'usuari ja té l'app oberta, COM s'hi enganxa i no
# se'n tanca res.
function Remember-Office([string]$exe) {
  $before = @(Get-Process $exe -ErrorAction SilentlyContinue | ForEach-Object { $_.Id })
  return $before
}
function Write-NewPid([string]$exe, $before) {
  $new = @(Get-Process $exe -ErrorAction SilentlyContinue | Where-Object { $before -notcontains $_.Id } | ForEach-Object { $_.Id })
  if ($new.Count -gt 0) { Set-Content -Path (Join-Path $OutDir 'office.pid') -Value ($new -join ' ') }
}
switch ($Kind) {
 'pptx' {
  $before = Remember-Office 'POWERPNT'
  $app = New-Object -ComObject PowerPoint.Application
  Write-NewPid 'POWERPNT' $before
  try {
   $pres = $app.Presentations.Open($Src, -1, 0, 0)
   $n = $pres.Slides.Count
   Write-Output "PAGES $n"
   $h = [int]($Width * $pres.PageSetup.SlideHeight / $pres.PageSetup.SlideWidth)
   $list = @(); if ($Pages -ne '') { $list = $Pages.Split(':') | ForEach-Object { [int]$_ } } else { $list = 1..$n }
   foreach ($i in $list) { if ($i -ge 1 -and $i -le $n) { $pres.Slides.Item($i).Export((Join-Path $OutDir "page-$i.png"), 'PNG', $Width, $h); Write-Output "OK $i" } }
   $pres.Close()
  } finally { $app.Quit() }
 }
 'docx' {
  $before = Remember-Office 'WINWORD'
  $app = New-Object -ComObject Word.Application
  Write-NewPid 'WINWORD' $before
  $app.Visible = $false; $app.DisplayAlerts = 0
  try { $d = $app.Documents.Open($Src, $false, $true); $d.SaveAs2([string]$pdf, 17); $d.Close($false); Write-Output "PDF $pdf" } finally { $app.Quit() }
 }
 'xlsx' {
  $before = Remember-Office 'EXCEL'
  $app = New-Object -ComObject Excel.Application
  Write-NewPid 'EXCEL' $before
  $app.Visible = $false; $app.DisplayAlerts = $false
  try { $w = $app.Workbooks.Open($Src, 0, $true); $w.ExportAsFixedFormat(0, [string]$pdf); $w.Close($false); Write-Output "PDF $pdf" } finally { $app.Quit() }
 }
}
`

func powershell() string {
	if p, err := exec.LookPath("powershell.exe"); err == nil {
		return p
	}
	if root := os.Getenv("SystemRoot"); root != "" {
		p := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func runPS(ctx context.Context, timeout time.Duration, script string, args ...string) (string, error) {
	ps := powershell()
	if ps == "" {
		return "", fmt.Errorf("powershell.exe no trobat")
	}
	f, err := os.CreateTemp("", "gregal-*.ps1")
	if err != nil {
		return "", err
	}
	// BOM: Windows PowerShell 5.1 llegeix els .ps1 sense BOM com a ANSI.
	f.Write([]byte{0xEF, 0xBB, 0xBF})
	f.WriteString(script)
	f.Close()
	defer os.Remove(f.Name())
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, ps, append([]string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", f.Name()}, args...)...)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("temps esgotat (%s)", timeout)
	}
	if err != nil {
		return string(out), fmt.Errorf("%v: %s", err, lastLines(string(out), 6))
	}
	return string(out), nil
}

func lastLines(s string, n int) string {
	ls := strings.Split(strings.TrimSpace(s), "\n")
	if len(ls) > n {
		ls = ls[len(ls)-n:]
	}
	return strings.Join(ls, "\n")
}

func joinInts(xs []int) string {
	var s []string
	for _, x := range xs {
		s = append(s, strconv.Itoa(x))
	}
	return strings.Join(s, ",")
}

func winPDF(ctx context.Context, pdf string, pages []int, o Options) ([]string, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("només a Windows")
	}
	if _, err := runPS(ctx, 120*time.Second, winPDFScript, "-Pdf", pdf, "-OutDir", o.OutDir, "-Width", strconv.Itoa(o.Width), "-Pages", strings.ReplaceAll(joinInts(pages), ",", ":")); err != nil {
		return nil, err
	}
	var out []string
	for _, n := range pages {
		p := filepath.Join(o.OutDir, fmt.Sprintf("page-%d.png", n))
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("no s'ha generat la pàgina %d", n)
		}
		out = append(out, p)
	}
	return out, nil
}

// officeCOM diu si hi ha Microsoft Office instal·lat (camins habituals).
func officeCOM() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)"} {
		d := os.Getenv(env)
		if d == "" {
			continue
		}
		for _, v := range []string{"root\\Office16", "Office16", "root\\Office15", "Office15"} {
			p := filepath.Join(d, "Microsoft Office", v, "POWERPNT.EXE")
			if _, err := os.Stat(p); err == nil {
				return filepath.Dir(p)
			}
			p = filepath.Join(d, "Microsoft Office", v, "WINWORD.EXE")
			if _, err := os.Stat(p); err == nil {
				return filepath.Dir(p)
			}
		}
	}
	return ""
}

func renderCOM(ctx context.Context, abs, ext string, o Options) (*Result, error) {
	if officeCOM() == "" {
		return nil, ErrNoEngine
	}
	kind := ""
	switch ext {
	case ".pptx", ".potx", ".pptm", ".ppt":
		kind = "pptx"
	case ".docx", ".dotx", ".docm", ".doc":
		kind = "docx"
	case ".xlsx", ".xltx", ".xlsm", ".xls":
		kind = "xlsx"
	default:
		return nil, fmt.Errorf("format no suportat: %s", ext)
	}
	pages := o.Pages
	out, err := runPS(ctx, o.Timeout, comScript, "-Src", abs, "-OutDir", o.OutDir, "-Kind", kind, "-Width", strconv.Itoa(o.Width), "-Pages", strings.ReplaceAll(joinInts(pages), ",", ":"))
	if err != nil {
		killOfficeOrphan(o.OutDir)
		return nil, fmt.Errorf("Microsoft Office: %v", err)
	}
	res := &Result{}
	if kind == "pptx" {
		res.Engine = "powerpoint"
		for _, l := range strings.Split(out, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "PAGES ") {
				res.Total, _ = strconv.Atoi(strings.TrimPrefix(l, "PAGES "))
			}
		}
		sel := choosePages(o, res.Total)
		for _, n := range sel {
			p := filepath.Join(o.OutDir, fmt.Sprintf("page-%d.png", n))
			if _, err := os.Stat(p); err == nil {
				res.Pages = append(res.Pages, p)
			}
		}
		return finish(res, o, sel), nil
	}
	pdf := filepath.Join(o.OutDir, "src.pdf")
	res.PDF = pdf
	res.Total = pdfPageCount(pdf)
	sel := choosePages(o, res.Total)
	pngs, err := winPDF(ctx, pdf, sel, o)
	if err != nil {
		return nil, err
	}
	res.Engine = map[string]string{"docx": "word+winpdf", "xlsx": "excel+winpdf"}[kind]
	res.Pages = pngs
	return finish(res, o, sel), nil
}

// killOfficeOrphan tanca la instància d'Office que ha obert comScript (el
// PID és a office.pid) quan l'exportació falla o es talla per temps: com
// que ve de DCOM, matar el PowerShell no la tancava i quedava un WINWORD o
// un EXCEL penjat. Si l'script s'havia enganxat a una app que l'usuari ja
// tenia oberta, no n'hi ha cap PID i no es toca res.
func killOfficeOrphan(outDir string) {
	raw, err := os.ReadFile(filepath.Join(outDir, "office.pid"))
	if err != nil {
		return
	}
	for _, f := range strings.Fields(string(raw)) {
		pid, err := strconv.Atoi(f)
		if err != nil || pid <= 0 {
			continue
		}
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Kill()
		}
	}
}
