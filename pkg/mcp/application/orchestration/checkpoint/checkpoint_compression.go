package checkpoint

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// compressData compresses data using the configured compression mode
func (cm *BoltCheckpointManager) compressData(data []byte) ([]byte, bool, error) {
	if cm.compressionMode == NoCompression {
		return data, false, nil
	}

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)

	_, err := gzWriter.Write(data)
	if err != nil {
		return nil, false, errors.NewError().Messagef("error").Build()
	}

	err = gzWriter.Close()
	if err != nil {
		return nil, false, errors.NewError().Messagef("error").Build()
	}

	compressed := buf.Bytes()

	if len(compressed) >= len(data) {
		cm.logger.Debug().
			Int("original_size", len(data)).
			Int("compressed_size", len(compressed)).
			Msg("Compression didn't reduce size, storing uncompressed")
		return data, false, nil
	}

	cm.logger.Debug().
		Int("original_size", len(data)).
		Int("compressed_size", len(compressed)).
		Float64("compression_ratio", float64(len(compressed))/float64(len(data))).
		Msg("Data compressed successfully")

	return compressed, true, nil
}

// decompressData decompresses data if it was compressed
func (cm *BoltCheckpointManager) decompressData(data []byte, isCompressed bool) ([]byte, error) {
	if !isCompressed {
		return data, nil
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, errors.NewError().Messagef("error").Build()
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, errors.NewError().Messagef("error").Build()
	}

	return decompressed, nil
}

// calculateChecksum calculates SHA-256 checksum of data
func (cm *BoltCheckpointManager) calculateChecksum(data []byte) string {
	if !cm.enableIntegrity {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// verifyChecksum verifies data integrity using checksum
func (cm *BoltCheckpointManager) verifyChecksum(data []byte, expectedChecksum string) error {
	if !cm.enableIntegrity || expectedChecksum == "" {
		return nil
	}

	actualChecksum := cm.calculateChecksum(data)
	if actualChecksum != expectedChecksum {
		return errors.NewError().Messagef("error").Build()
	}

	return nil
}
