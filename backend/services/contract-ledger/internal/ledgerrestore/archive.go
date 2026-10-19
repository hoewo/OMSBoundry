package ledgerrestore

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/ledgerexport"
)

const (
	manifestFilename = "manifest.json"
	ledgerFilename   = "ledger.json"
	maxManifestBytes = int64(1 << 20)
	maxLedgerBytes   = int64(32 << 20)
)

func ParseArchive(content []byte) (Candidate, error) {
	if len(content) == 0 {
		return Candidate{}, invalid("archive", "必须选择 Contract Ledger ZIP 归档")
	}
	if int64(len(content)) > MaxArchiveBytes {
		return Candidate{}, &TooLargeError{Message: "恢复归档不能超过 64 MiB"}
	}
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return Candidate{}, invalid("archive", "恢复归档不是有效 ZIP")
	}
	if len(reader.File) != 2 {
		return Candidate{}, invalid("archive", "恢复归档必须且只能包含 manifest.json 和 ledger.json")
	}
	files := make(map[string][]byte, 2)
	for _, file := range reader.File {
		name := file.Name
		if name != manifestFilename && name != ledgerFilename {
			return Candidate{}, invalid("archive", "恢复归档包含未知路径或文件")
		}
		if path.Clean(name) != name || strings.Contains(name, "\\") || strings.Contains(name, "/") {
			return Candidate{}, invalid("archive", "恢复归档文件必须位于根目录")
		}
		if _, exists := files[name]; exists {
			return Candidate{}, invalid("archive", "恢复归档包含重复文件")
		}
		if file.Flags&0x1 != 0 {
			return Candidate{}, invalid("archive", "恢复归档不能包含加密文件")
		}
		if file.FileInfo().IsDir() || !file.Mode().IsRegular() {
			return Candidate{}, invalid("archive", "恢复归档只能包含普通文件")
		}
		limit := maxLedgerBytes
		if name == manifestFilename {
			limit = maxManifestBytes
		}
		if file.UncompressedSize64 > uint64(limit) {
			return Candidate{}, &TooLargeError{Message: name + " 解压后超过大小限制"}
		}
		value, err := readZipFile(file, limit)
		if err != nil {
			var tooLarge *TooLargeError
			if errors.As(err, &tooLarge) {
				return Candidate{}, err
			}
			return Candidate{}, invalid("archive", "读取恢复归档失败")
		}
		files[name] = value
	}
	manifestBytes, manifestFound := files[manifestFilename]
	ledgerBytes, ledgerFound := files[ledgerFilename]
	if !manifestFound || !ledgerFound {
		return Candidate{}, invalid("archive", "恢复归档缺少 manifest.json 或 ledger.json")
	}

	var manifest ledgerexport.Manifest
	if err := decodeStrictJSON(manifestBytes, &manifest); err != nil {
		return Candidate{}, invalid("manifest", "manifest.json 格式无效或包含未知字段")
	}
	if manifest.FormatID != ledgerexport.FormatID || manifest.FormatVersion != ledgerexport.FormatVersion {
		return Candidate{}, invalid("manifest", "恢复归档格式或版本不受支持")
	}
	generatedAt, err := time.Parse(time.RFC3339Nano, manifest.GeneratedAt)
	if err != nil {
		return Candidate{}, invalid("manifest.generated_at", "manifest 生成时间无效")
	}
	if !validUUID(manifest.ContractID) {
		return Candidate{}, invalid("manifest.contract_id", "manifest 合同标识无效")
	}
	if manifest.LedgerFilename != ledgerFilename || manifest.EvidenceContentIncluded {
		return Candidate{}, invalid("manifest", "manifest 的账簿文件或 Evidence 内容声明无效")
	}
	if manifest.LedgerBytes != len(ledgerBytes) {
		return Candidate{}, invalid("manifest.ledger_bytes", "ledger.json 字节数与 manifest 不一致")
	}
	digestBytes := sha256.Sum256(ledgerBytes)
	digest := hex.EncodeToString(digestBytes[:])
	if manifest.LedgerSHA256 != digest {
		return Candidate{}, invalid("manifest.ledger_sha256", "ledger.json SHA-256 与 manifest 不一致")
	}

	var ledger ledgerexport.Ledger
	if err := decodeStrictJSON(ledgerBytes, &ledger); err != nil {
		return Candidate{}, invalid("ledger", "ledger.json 格式无效或包含未知字段")
	}
	if ledger.Format != ledgerexport.FormatID {
		return Candidate{}, invalid("ledger.format", "ledger 格式标识无效")
	}
	if ledger.Contract.ID != strings.ToLower(manifest.ContractID) {
		return Candidate{}, invalid("ledger.contract.id", "manifest 与 ledger 的合同标识不一致")
	}
	if len(ledger.Relations) != 0 {
		return Candidate{}, &UnsupportedRelationsError{Count: len(ledger.Relations)}
	}
	if err := validateLedger(ledger); err != nil {
		return Candidate{}, err
	}
	return Candidate{
		Manifest: manifest, Ledger: ledger, LedgerSHA256: digest,
		SourceGeneratedAt: generatedAt.UTC(),
	}, nil
}

func readZipFile(file *zip.File, limit int64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	value, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(value)) > limit {
		return nil, &TooLargeError{Message: file.Name + " 解压后超过大小限制"}
	}
	return value, nil
}

func decodeStrictJSON(content []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
