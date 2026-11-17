package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/sqlite"
)

type Part struct {
	ID            int64
	BHSNo         string
	DescriptionCN string
	DescriptionEN string
	Spec          string
	Manufacturer  string
	OrderNo       string
	Category      string
	Subcategory   string
	Unit          string
	Dispo         string
	Cert          string
	Machine       string
	ImportFlag    string
	Remark        string
	Datasheet     string
	Price         string
	SpareLevel    string
	ActiveFlag    string
	SLOB          string
	SafetyStock   string
	ProtocolStock string
	Stock         string
	MOQ           string
	Delivery      string
	AlternateNo   string
	RelatedPart   string
	CNPartNo      string
	DonganPartNo  string
	SPartNo       string
	Status        string
	Origin        string
	SearchText    string
	ExtraJSON     string
	SourceFile    string
	SourceSheet   string
	SourceRow     int64
	CreatedAt     string
	UpdatedAt     string
	Duplicate     int
}

func (p *Part) Name() string {
	if p.DescriptionCN != "" {
		return p.DescriptionCN
	}
	if p.DescriptionEN != "" {
		return p.DescriptionEN
	}
	return "未命名元件"
}

func (p *Part) SecondaryName() string {
	if p.DescriptionCN != "" && p.DescriptionEN != "" && p.DescriptionCN != p.DescriptionEN {
		return p.DescriptionEN
	}
	return ""
}

func (p *Part) SourceLabel() string {
	if p.SourceFile == "" {
		return "手动维护"
	}
	sheet := p.SourceSheet
	if sheet == "" {
		sheet = p.Subcategory
	}
	label := sheet
	if label == "" {
		label = "未记录"
	}
	if p.Origin == "manual" {
		label += " · 手动"
	}
	return label
}

func (p *Part) SearchBlob() string {
	if p.SearchText != "" {
		return p.SearchText
	}
	return strings.ToLower(strings.Join([]string{
		p.BHSNo, p.DescriptionCN, p.DescriptionEN, p.Spec, p.Manufacturer,
		p.OrderNo, p.Category, p.Subcategory, p.Cert, p.Machine, p.Remark,
		p.AlternateNo, p.RelatedPart, p.CNPartNo, p.DonganPartNo, p.SPartNo,
	}, " "))
}

type SheetInfo struct {
	Name  string
	Count int
}

type WorkbookInfo struct {
	Category   string
	SourceFile string
	Count      int
	Sheets     []SheetInfo
}

type Summary struct {
	Total         int
	BHSCodes      int
	Active        int
	Inactive      int
	Historical    int
	Categories    int
	Manufacturers int
	Duplicates    int
	ImportDate    string
	Files         int
	Sheets        int
}

type Store struct {
	db *sqlite.DB
}

func OpenStore(ctx context.Context, path string) (*Store, error) {
	db, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) LoadAll(ctx context.Context) ([]*Part, []WorkbookInfo, Summary, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, bhs_no, description_cn, description_en, spec, manufacturer,
		       order_no, category, subcategory, unit, dispo, cert, machine,
		       import_flag, remark, datasheet, price, spare_level, active_flag,
		       slob, safety_stock, protocol_stock, stock, moq, delivery,
		       alternate_no, related_part, cn_part_no, dongan_part_no,
		       s_part_no, status, origin, search_text, extra_json,
		       source_file, source_sheet, source_row, created_at, updated_at
		FROM parts
		ORDER BY id
	`)
	if err != nil {
		return nil, nil, Summary{}, err
	}
	index := columnIndex(rows.Columns)
	parts := make([]*Part, 0, len(rows.Values))
	for _, values := range rows.Values {
		p := &Part{
			ID:            intValue(values, index, "id"),
			BHSNo:         stringValue(values, index, "bhs_no"),
			DescriptionCN: stringValue(values, index, "description_cn"),
			DescriptionEN: stringValue(values, index, "description_en"),
			Spec:          stringValue(values, index, "spec"),
			Manufacturer:  stringValue(values, index, "manufacturer"),
			OrderNo:       stringValue(values, index, "order_no"),
			Category:      stringValue(values, index, "category"),
			Subcategory:   stringValue(values, index, "subcategory"),
			Unit:          stringValue(values, index, "unit"),
			Dispo:         stringValue(values, index, "dispo"),
			Cert:          stringValue(values, index, "cert"),
			Machine:       stringValue(values, index, "machine"),
			ImportFlag:    stringValue(values, index, "import_flag"),
			Remark:        stringValue(values, index, "remark"),
			Datasheet:     stringValue(values, index, "datasheet"),
			Price:         stringValue(values, index, "price"),
			SpareLevel:    stringValue(values, index, "spare_level"),
			ActiveFlag:    stringValue(values, index, "active_flag"),
			SLOB:          stringValue(values, index, "slob"),
			SafetyStock:   stringValue(values, index, "safety_stock"),
			ProtocolStock: stringValue(values, index, "protocol_stock"),
			Stock:         stringValue(values, index, "stock"),
			MOQ:           stringValue(values, index, "moq"),
			Delivery:      stringValue(values, index, "delivery"),
			AlternateNo:   stringValue(values, index, "alternate_no"),
			RelatedPart:   stringValue(values, index, "related_part"),
			CNPartNo:      stringValue(values, index, "cn_part_no"),
			DonganPartNo:  stringValue(values, index, "dongan_part_no"),
			SPartNo:       stringValue(values, index, "s_part_no"),
			Status:        stringValue(values, index, "status"),
			Origin:        stringValue(values, index, "origin"),
			SearchText:    stringValue(values, index, "search_text"),
			ExtraJSON:     stringValue(values, index, "extra_json"),
			SourceFile:    stringValue(values, index, "source_file"),
			SourceSheet:   stringValue(values, index, "source_sheet"),
			SourceRow:     intValue(values, index, "source_row"),
			CreatedAt:     stringValue(values, index, "created_at"),
			UpdatedAt:     stringValue(values, index, "updated_at"),
		}
		parts = append(parts, p)
	}

	byBHS := make(map[string]int)
	for _, p := range parts {
		if p.BHSNo != "" {
			byBHS[p.BHSNo]++
		}
	}
	for _, p := range parts {
		p.Duplicate = byBHS[p.BHSNo]
	}

	workbooks, err := s.loadWorkbooks(ctx)
	if err != nil {
		return nil, nil, Summary{}, err
	}
	meta, err := s.loadMeta(ctx)
	if err != nil {
		return nil, nil, Summary{}, err
	}
	summary := summarize(parts, workbooks, meta)
	return parts, workbooks, summary, nil
}

func (s *Store) loadWorkbooks(ctx context.Context) ([]WorkbookInfo, error) {
	rows, err := s.db.Query(ctx, `
		SELECT ss.category, ss.source_file, ss.source_sheet,
		       COUNT(p.id) AS count
		FROM source_sheets ss
		LEFT JOIN parts p
		  ON p.source_file = ss.source_file
		 AND p.source_sheet = ss.source_sheet
		GROUP BY ss.category, ss.source_file, ss.source_sheet
		ORDER BY ss.category COLLATE NOCASE, ss.source_sheet COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	index := columnIndex(rows.Columns)
	byFile := map[string]*WorkbookInfo{}
	order := []string{}
	for _, values := range rows.Values {
		file := stringValue(values, index, "source_file")
		wb := byFile[file]
		if wb == nil {
			wb = &WorkbookInfo{
				Category:   stringValue(values, index, "category"),
				SourceFile: file,
			}
			byFile[file] = wb
			order = append(order, file)
		}
		count := int(intValue(values, index, "count"))
		wb.Count += count
		wb.Sheets = append(wb.Sheets, SheetInfo{
			Name:  stringValue(values, index, "source_sheet"),
			Count: count,
		})
	}
	workbooks := make([]WorkbookInfo, 0, len(order))
	for _, file := range order {
		workbooks = append(workbooks, *byFile[file])
	}
	slices.SortFunc(workbooks, func(a, b WorkbookInfo) int {
		return strings.Compare(strings.ToLower(a.Category), strings.ToLower(b.Category))
	})
	return workbooks, nil
}

func (s *Store) loadMeta(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.Query(ctx, "SELECT key, value FROM meta")
	if err != nil {
		return nil, err
	}
	index := columnIndex(rows.Columns)
	meta := make(map[string]string, len(rows.Values))
	for _, values := range rows.Values {
		meta[stringValue(values, index, "key")] = stringValue(values, index, "value")
	}
	return meta, nil
}

func summarize(parts []*Part, workbooks []WorkbookInfo, meta map[string]string) Summary {
	codes := map[string]struct{}{}
	categories := map[string]struct{}{}
	manufacturers := map[string]struct{}{}
	duplicateBHS := map[string]struct{}{}
	summary := Summary{Total: len(parts)}
	for _, p := range parts {
		if p.BHSNo != "" {
			codes[p.BHSNo] = struct{}{}
			if p.Duplicate > 1 {
				duplicateBHS[p.BHSNo] = struct{}{}
			}
		}
		if p.Category != "" {
			categories[p.Category] = struct{}{}
		}
		if p.Manufacturer != "" {
			manufacturers[p.Manufacturer] = struct{}{}
		}
		switch p.Status {
		case "active":
			summary.Active++
		case "inactive":
			summary.Inactive++
		default:
			summary.Historical++
		}
	}
	summary.BHSCodes = len(codes)
	summary.Categories = len(categories)
	summary.Manufacturers = len(manufacturers)
	summary.Duplicates = len(duplicateBHS)
	summary.ImportDate = meta["imported_at"]
	summary.Files = len(workbooks)
	for _, wb := range workbooks {
		summary.Sheets += len(wb.Sheets)
	}
	return summary
}

func (s *Store) SavePart(ctx context.Context, p *Part, isNew bool) (int64, error) {
	now := time.Now().Format(time.RFC3339)
	p.SearchText = partSearchText(p)
	if p.Origin == "" {
		p.Origin = "manual"
	}
	fields := []string{
		"bhs_no", "description_cn", "description_en", "spec", "manufacturer",
		"order_no", "category", "subcategory", "unit", "dispo", "cert",
		"machine", "import_flag", "remark", "datasheet", "price",
		"spare_level", "active_flag", "slob", "safety_stock",
		"protocol_stock", "stock", "moq", "delivery", "alternate_no",
		"related_part", "cn_part_no", "dongan_part_no", "s_part_no",
		"status", "origin", "search_text", "extra_json", "source_file",
		"source_sheet", "updated_at",
	}
	args := []any{
		p.BHSNo, p.DescriptionCN, p.DescriptionEN, p.Spec, p.Manufacturer,
		p.OrderNo, p.Category, p.Subcategory, p.Unit, p.Dispo, p.Cert,
		p.Machine, p.ImportFlag, p.Remark, p.Datasheet, p.Price,
		p.SpareLevel, p.ActiveFlag, p.SLOB, p.SafetyStock,
		p.ProtocolStock, p.Stock, p.MOQ, p.Delivery, p.AlternateNo,
		p.RelatedPart, p.CNPartNo, p.DonganPartNo, p.SPartNo,
		p.Status, p.Origin, p.SearchText, p.ExtraJSON, p.SourceFile,
		p.SourceSheet, now,
	}
	var result sqlite.Result
	var err error
	if isNew {
		p.CreatedAt = now
		fields = append(fields, "created_at")
		args = append(args, now)
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(fields)), ",")
		result, err = s.db.Execute(ctx,
			fmt.Sprintf("INSERT INTO parts (%s) VALUES (%s)", strings.Join(fields, ","), placeholders),
			args...,
		)
		if err != nil {
			return 0, err
		}
		p.ID = result.LastInsertID
	} else {
		assignments := make([]string, len(fields))
		for i, field := range fields {
			assignments[i] = field + " = ?"
		}
		args = append(args, p.ID)
		result, err = s.db.Execute(ctx,
			fmt.Sprintf("UPDATE parts SET %s WHERE id = ?", strings.Join(assignments, ",")),
			args...,
		)
		if err != nil {
			return 0, err
		}
		if result.Changes == 0 {
			return 0, fmt.Errorf("记录不存在")
		}
	}
	action, title := "update", "修改 "+p.BHSNo
	if isNew {
		action, title = "create", "新增 "+p.BHSNo
	}
	_, _ = s.db.Execute(ctx, `
		INSERT INTO activity (part_id, action, title, detail, operator, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, p.ID, action, title, "MyGo 原生界面操作", "Demo 维护员", now)
	return p.ID, nil
}

func columnIndex(columns []string) map[string]int {
	index := make(map[string]int, len(columns))
	for i, name := range columns {
		index[name] = i
	}
	return index
}

func stringValue(values []any, index map[string]int, name string) string {
	i, ok := index[name]
	if !ok || i >= len(values) || values[i] == nil {
		return ""
	}
	if s, ok := values[i].(string); ok {
		return s
	}
	return fmt.Sprint(values[i])
}

func intValue(values []any, index map[string]int, name string) int64 {
	i, ok := index[name]
	if !ok || i >= len(values) || values[i] == nil {
		return 0
	}
	switch v := values[i].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func partSearchText(p *Part) string {
	return strings.ToLower(strings.Join([]string{
		p.BHSNo, p.DescriptionCN, p.DescriptionEN, p.Spec, p.Manufacturer,
		p.OrderNo, p.Category, p.Subcategory, p.Unit, p.Dispo, p.Cert,
		p.Machine, p.Remark, p.AlternateNo, p.RelatedPart, p.CNPartNo,
		p.DonganPartNo, p.SPartNo,
	}, " "))
}
