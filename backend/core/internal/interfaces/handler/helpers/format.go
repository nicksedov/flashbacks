package helpers

import "strconv"

// FormatSize formats file size in human readable format.
func FormatSize(size int64) string {
	const unit = 1024
	if size < unit {
		return strconv.FormatInt(size, 10) + " B"
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	// Equivalent to fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp]);
	// strconv avoids the reflect-based formatting path on this per-row hot path.
	buf := strconv.AppendFloat(make([]byte, 0, 16), float64(size)/float64(div), 'f', 1, 64)
	buf = append(buf, ' ', "KMGTPE"[exp], 'B')
	return string(buf)
}
