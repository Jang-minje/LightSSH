Add-Type -AssemblyName System.Drawing
$base = Join-Path (Get-Location) 'assets'
$sizes = @(16,24,32,48,64,128,256)
$pngs = @()
foreach ($size in $sizes) {
  $bmp = New-Object System.Drawing.Bitmap $size, $size, ([System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
  $g.Clear([System.Drawing.Color]::Transparent)
  $rect = New-Object System.Drawing.RectangleF 0,0,$size,$size
  $path = New-Object System.Drawing.Drawing2D.GraphicsPath
  $radius = [Math]::Max(3, $size * 0.18)
  function Add-RoundRect($p, [float]$x, [float]$y, [float]$w, [float]$h, [float]$r) {
    $d = $r * 2
    $p.AddArc($x, $y, $d, $d, 180, 90)
    $p.AddArc($x + $w - $d, $y, $d, $d, 270, 90)
    $p.AddArc($x + $w - $d, $y + $h - $d, $d, $d, 0, 90)
    $p.AddArc($x, $y + $h - $d, $d, $d, 90, 90)
    $p.CloseFigure()
  }
  Add-RoundRect $path ($size*0.08) ($size*0.08) ($size*0.84) ($size*0.84) $radius
  $bg = [System.Drawing.Drawing2D.LinearGradientBrush]::new(
    $rect,
    [System.Drawing.Color]::FromArgb(255,16,24,32),
    [System.Drawing.Color]::FromArgb(255,47,111,237),
    [single]45
  )
  $g.FillPath($bg, $path)
  $border = New-Object System.Drawing.Pen ([System.Drawing.Color]::FromArgb(210,124,169,255)), ([Math]::Max(1, $size*0.025))
  $g.DrawPath($border, $path)

  $green = New-Object System.Drawing.SolidBrush ([System.Drawing.Color]::FromArgb(255,31,182,91))
  $g.FillEllipse($green, $size*0.16, $size*0.16, $size*0.13, $size*0.13)

  $pen = New-Object System.Drawing.Pen ([System.Drawing.Color]::FromArgb(255,230,237,243)), ([Math]::Max(2, $size*0.07))
  $pen.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
  $pen.EndCap = [System.Drawing.Drawing2D.LineCap]::Round
  $g.DrawLines($pen, @(
    (New-Object System.Drawing.PointF ($size*0.30), ($size*0.37)),
    (New-Object System.Drawing.PointF ($size*0.45), ($size*0.50)),
    (New-Object System.Drawing.PointF ($size*0.30), ($size*0.63))
  ))
  $cursorPen = New-Object System.Drawing.Pen ([System.Drawing.Color]::FromArgb(255,230,237,243)), ([Math]::Max(2, $size*0.07))
  $cursorPen.StartCap = [System.Drawing.Drawing2D.LineCap]::Round
  $cursorPen.EndCap = [System.Drawing.Drawing2D.LineCap]::Round
  $g.DrawLine($cursorPen, $size*0.52, $size*0.63, $size*0.72, $size*0.63)

  $pngPath = Join-Path $base "lightssh-$size.png"
  $bmp.Save($pngPath, [System.Drawing.Imaging.ImageFormat]::Png)
  $pngs += [PSCustomObject]@{Size=$size; Path=$pngPath}
  $g.Dispose(); $bmp.Dispose()
}
Copy-Item (Join-Path $base 'lightssh-256.png') (Join-Path $base 'lightssh.png') -Force

$icoPath = Join-Path $base 'lightssh.ico'
$fs = [System.IO.File]::Open($icoPath, [System.IO.FileMode]::Create)
$bw = New-Object System.IO.BinaryWriter $fs
$bw.Write([UInt16]0)
$bw.Write([UInt16]1)
$bw.Write([UInt16]$pngs.Count)
$offset = 6 + (16 * $pngs.Count)
$items = @()
foreach ($png in $pngs) {
  $bytes = [System.IO.File]::ReadAllBytes($png.Path)
  $items += [PSCustomObject]@{Size=$png.Size; Bytes=$bytes; Offset=$offset}
  $widthByte = if ($png.Size -eq 256) { 0 } else { $png.Size }
  $bw.Write([Byte]$widthByte)
  $bw.Write([Byte]$widthByte)
  $bw.Write([Byte]0)
  $bw.Write([Byte]0)
  $bw.Write([UInt16]1)
  $bw.Write([UInt16]32)
  $bw.Write([UInt32]$bytes.Length)
  $bw.Write([UInt32]$offset)
  $offset += $bytes.Length
}
foreach ($item in $items) { $bw.Write($item.Bytes) }
$bw.Close(); $fs.Close()
