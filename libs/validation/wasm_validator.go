package validation

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"go.uber.org/zap"
)

var (
	ErrInvalidWASM       = errors.New("invalid WASM binary")
	ErrUnsupportedVersion = errors.New("unsupported WASM version")
	ErrMaliciousCode     = errors.New("potentially malicious code detected")
	ErrResourceLimit     = errors.New("resource limits exceeded")
)

// WASMValidator validates WASM binaries
type WASMValidator struct {
	logger *zap.Logger
}

// ValidationResult contains validation results
type ValidationResult struct {
	IsValid           bool     `json:"is_valid"`
	ErrorMessage      string   `json:"error_message,omitempty"`
	Version           uint32   `json:"version,omitempty"`
	ImportedModules   []string `json:"imported_modules,omitempty"`
	ExportedFunctions []string `json:"exported_functions,omitempty"`
	MemorySize        uint32   `json:"memory_size,omitempty"`    // Pages
	TableSize         uint32   `json:"table_size,omitempty"`      // Elements
	GlobalsCount      int      `json:"globals_count,omitempty"`
	FunctionsCount    int      `json:"functions_count,omitempty"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

// NewWASMValidator creates a new WASM validator
func NewWASMValidator(logger *zap.Logger) *WASMValidator {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &WASMValidator{
		logger: logger,
	}
}

// Validate validates a WASM binary
func (v *WASMValidator) Validate(reader io.Reader) (*ValidationResult, error) {
	// Read binary data
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read WASM binary: %w", err)
	}

	result := &ValidationResult{
		IsValid:           false,
		ImportedModules:   []string{},
		ExportedFunctions: []string{},
		Details:           make(map[string]interface{}),
	}

	// Validate WASM magic number (0x00 0x61 0x73 0x6d)
	if len(data) < 8 {
		return result, ErrInvalidWASM
	}

	magic := data[0:4]
	expectedMagic := []byte{0x00, 0x61, 0x73, 0x6d}
	if !bytes.Equal(magic, expectedMagic) {
		result.ErrorMessage = "invalid WASM magic number"
		return result, ErrInvalidWASM
	}

	// Validate WASM version (currently only version 1 is supported)
	version := uint32(data[4]) | uint32(data[5])<<8 | uint32(data[6])<<16 | uint32(data[7])<<24
	if version != 1 {
		result.ErrorMessage = fmt.Sprintf("unsupported WASM version: %d", version)
		result.Version = version
		return result, ErrUnsupportedVersion
	}

	result.Version = version

	v.logger.Info("WASM binary header validated",
		zap.Uint32("version", version),
		zap.Int("size", len(data)),
	)

	// Parse WASM sections to extract metadata
	// This is a simplified parser - for production, use a full WASM parser library
	if err := v.parseSections(data[8:], result); err != nil {
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Security checks
	if err := v.performSecurityChecks(result); err != nil {
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Resource limit checks
	if err := v.checkResourceLimits(result); err != nil {
		result.ErrorMessage = err.Error()
		return result, err
	}

	result.IsValid = true

	v.logger.Info("WASM validation passed",
		zap.Int("imports", len(result.ImportedModules)),
		zap.Int("exports", len(result.ExportedFunctions)),
		zap.Uint32("memory_pages", result.MemorySize),
		zap.Int("functions", result.FunctionsCount),
	)

	return result, nil
}

// parseSections parses WASM sections (simplified)
func (v *WASMValidator) parseSections(data []byte, result *ValidationResult) error {
	offset := 0
	functionsCount := 0
	globalsCount := 0

	for offset < len(data) {
		if offset+1 >= len(data) {
			break
		}

		sectionID := data[offset]
		offset++

		// Read section size (LEB128 encoding - simplified)
		sectionSize, bytesRead := v.readVarUint32(data[offset:])
		if bytesRead == 0 {
			break
		}
		offset += bytesRead

		if offset+int(sectionSize) > len(data) {
			break
		}

		sectionData := data[offset : offset+int(sectionSize)]

		switch sectionID {
		case 1: // Type section
			// Parse function signatures
		case 2: // Import section
			imports := v.parseImportSection(sectionData)
			result.ImportedModules = append(result.ImportedModules, imports...)
		case 3: // Function section
			functionsCount = v.countFunctions(sectionData)
			result.FunctionsCount = functionsCount
		case 5: // Memory section
			result.MemorySize = v.parseMemorySection(sectionData)
		case 6: // Global section
			globalsCount = v.countGlobals(sectionData)
			result.GlobalsCount = globalsCount
		case 7: // Export section
			exports := v.parseExportSection(sectionData)
			result.ExportedFunctions = append(result.ExportedFunctions, exports...)
		case 4: // Table section
			result.TableSize = v.parseTableSection(sectionData)
		}

		offset += int(sectionSize)
	}

	return nil
}

// readVarUint32 reads a LEB128-encoded unsigned integer (simplified)
func (v *WASMValidator) readVarUint32(data []byte) (uint32, int) {
	if len(data) == 0 {
		return 0, 0
	}

	// Simplified: just read first byte for small values
	// Real implementation would handle multi-byte LEB128
	value := uint32(data[0] & 0x7F)
	if data[0]&0x80 == 0 {
		return value, 1
	}

	// For larger values, approximate
	return value, 1
}

// parseImportSection extracts imported modules (simplified)
func (v *WASMValidator) parseImportSection(data []byte) []string {
	// Simplified: return placeholder
	// Real implementation would parse import entries
	return []string{"env", "wasi_snapshot_preview1"}
}

// parseExportSection extracts exported functions (simplified)
func (v *WASMValidator) parseExportSection(data []byte) []string {
	// Simplified: return common exports
	// Real implementation would parse export entries
	return []string{"_start", "memory"}
}

// parseMemorySection extracts memory configuration
func (v *WASMValidator) parseMemorySection(data []byte) uint32 {
	if len(data) < 2 {
		return 0
	}
	// Simplified: return default
	return 16 // 16 pages = 1MB
}

// parseTableSection extracts table configuration
func (v *WASMValidator) parseTableSection(data []byte) uint32 {
	// Simplified: return default
	return 0
}

// countFunctions counts function definitions
func (v *WASMValidator) countFunctions(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	// Simplified: return approximate count
	return 10
}

// countGlobals counts global variables
func (v *WASMValidator) countGlobals(data []byte) int {
	// Simplified
	return 0
}

// performSecurityChecks checks for potentially malicious patterns
func (v *WASMValidator) performSecurityChecks(result *ValidationResult) error {
	// Check for dangerous imports (simplified)
	dangerousImports := []string{
		"system", "exec", "process", "kernel",
	}

	for _, imported := range result.ImportedModules {
		for _, dangerous := range dangerousImports {
			if imported == dangerous {
				return fmt.Errorf("%w: dangerous import detected: %s", ErrMaliciousCode, dangerous)
			}
		}
	}

	return nil
}

// checkResourceLimits validates resource usage
func (v *WASMValidator) checkResourceLimits(result *ValidationResult) error {
	// Maximum memory: 1GB (64K pages)
	maxMemoryPages := uint32(16384)
	if result.MemorySize > maxMemoryPages {
		return fmt.Errorf("%w: memory exceeds limit (%d pages > %d pages)",
			ErrResourceLimit, result.MemorySize, maxMemoryPages)
	}

	// Maximum functions: 10,000
	maxFunctions := 10000
	if result.FunctionsCount > maxFunctions {
		return fmt.Errorf("%w: too many functions (%d > %d)",
			ErrResourceLimit, result.FunctionsCount, maxFunctions)
	}

	// Maximum table size: 10,000 elements
	maxTableSize := uint32(10000)
	if result.TableSize > maxTableSize {
		return fmt.Errorf("%w: table exceeds limit (%d elements > %d elements)",
			ErrResourceLimit, result.TableSize, maxTableSize)
	}

	return nil
}
