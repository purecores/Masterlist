# AI 添加与同步 Excel 文档至 SQLite 数据库规则标准
# (AI Excel-to-SQL Import & Maintenance Specification)

本规范为后续大语言模型 (AI)、自动化脚本或开发人员向 BHS 元器件库 (`data/masterlist.db`) 导入、增量更新或追加新的 Excel 格式供应商清单及 Master E-list 表格时的**强制执行标准**。公开 Release 附带的是脱敏演示库 `data/masterlist.example.db`；生产环境应将业务库保存在 `data/masterlist.db`，二者的表结构完全一致。

---

## 1. 目标数据库架构规范 (Target Database Schema)

数据库引擎为标准 SQLite 3。必须遵循以下表结构与约束：

### 1.1 主元器件表: `parts`
```sql
CREATE TABLE parts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    bhs_no TEXT NOT NULL DEFAULT '',           -- BHS 内部零件号 (核心索引键，如 100917800)
    description_cn TEXT NOT NULL DEFAULT '',   -- 中文描述/元件名称 (如 "铜母排")
    description_en TEXT NOT NULL DEFAULT '',   -- 英文描述 (如 "COPPER LEDGE")
    spec TEXT NOT NULL DEFAULT '',             -- 规格/型号 (如 "250*400*2,5" 或 "WO-01618")
    manufacturer TEXT NOT NULL DEFAULT '',     -- 制造商/品牌 (如 "Schneider", "Siemens", "Woehner")
    order_no TEXT NOT NULL DEFAULT '',         -- 订货号/订货代码
    category TEXT NOT NULL DEFAULT '未分类',   -- 主分类 (如 "Accessories", "Circuit Breaker", "母线")
    subcategory TEXT NOT NULL DEFAULT '',      -- 子分类/工作表名称 (如 "Terminal", "Rail")
    unit TEXT NOT NULL DEFAULT '',             -- 单位 (默认为 "个" 或 "PC")
    dispo TEXT NOT NULL DEFAULT '',            -- 属性/Dispo/计划分类 (如 "E", "M7", "R")
    cert TEXT NOT NULL DEFAULT '',             -- 认证标准 (如 "UL", "CE", "CCC")
    machine TEXT NOT NULL DEFAULT '',          -- 适用机型/功能段 (如 "SF", "DF", "US")
    import_flag TEXT NOT NULL DEFAULT '',      -- WHR/进口标识 (如 "WHR", "进口", "国产")
    remark TEXT NOT NULL DEFAULT '',           -- 备注说明
    datasheet TEXT NOT NULL DEFAULT '',        -- 数据手册指定文件名 (可选)
    price TEXT NOT NULL DEFAULT '',            -- 参考单价/币种
    spare_level TEXT NOT NULL DEFAULT '',      -- 备件级别
    active_flag TEXT NOT NULL DEFAULT '',      -- 原表有效性标识
    slob TEXT NOT NULL DEFAULT '',             -- SLOB/呆滞物料标识
    safety_stock TEXT NOT NULL DEFAULT '',     -- 安全库存
    protocol_stock TEXT NOT NULL DEFAULT '',   -- 协议库存
    stock TEXT NOT NULL DEFAULT '',            -- 当前实际库存
    moq TEXT NOT NULL DEFAULT '',              -- 最小起订量 (MOQ)
    delivery TEXT NOT NULL DEFAULT '',         -- 供货周期/货期 (如 "4 weeks")
    alternate_no TEXT NOT NULL DEFAULT '',     -- 替代件号/旧零件号 (G-No.)
    related_part TEXT NOT NULL DEFAULT '',     -- 相关物料/组件关联号
    cn_part_no TEXT NOT NULL DEFAULT '',       -- 中国件号 (CN No.)
    dongan_part_no TEXT NOT NULL DEFAULT '',   -- 东安件号
    s_part_no TEXT NOT NULL DEFAULT '',        -- S 件号
    status TEXT NOT NULL DEFAULT 'active'      -- 状态，严格限制: 'active'(有效), 'inactive'(停用), 'obsolete'(历史)
        CHECK (status IN ('active', 'inactive', 'obsolete')),
    origin TEXT NOT NULL DEFAULT 'import'      -- 来源，严格限制: 'import'(表格导入), 'manual'(人工维护)
        CHECK (origin IN ('import', 'manual')),
    search_text TEXT NOT NULL DEFAULT '',      -- 预计算全文检索字段 (小写分词拼接)
    extra_json TEXT NOT NULL DEFAULT '{}',     -- 无法映射入标准列的其他扩展属性 (JSON 字典)
    source_file TEXT NOT NULL DEFAULT '',      -- 来源 Excel 文件名 (如 "Master E-list_Accessories.xlsx")
    source_sheet TEXT NOT NULL DEFAULT '',     -- 来源 Excel 工作表名 (如 "EMC|PE|ESD Protection")
    source_row INTEGER,                        -- 来源 Excel 所在物理行号 (从 1 起算)
    created_at TEXT NOT NULL,                  -- 创建时间 (ISO8601: YYYY-MM-DDTHH:MM:SSZ)
    updated_at TEXT NOT NULL                   -- 更新时间 (ISO8601: YYYY-MM-DDTHH:MM:SSZ)
);

CREATE INDEX idx_parts_bhs ON parts(bhs_no);
CREATE INDEX idx_parts_order ON parts(order_no);
CREATE INDEX idx_parts_category ON parts(category);
CREATE INDEX idx_parts_manufacturer ON parts(manufacturer);
CREATE INDEX idx_parts_status ON parts(status);
```

### 1.2 来源工作表登记表: `source_sheets`
用于在界面左侧与顶部精准还原原表树状与胶囊导航：
```sql
CREATE TABLE source_sheets (
    source_file TEXT NOT NULL,
    source_sheet TEXT NOT NULL,
    category TEXT NOT NULL,
    PRIMARY KEY (source_file, source_sheet)
);
```

### 1.3 导入批次日志表: `import_batches`
```sql
CREATE TABLE import_batches (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    finished_at TEXT NOT NULL,
    files_count INTEGER NOT NULL,
    sheets_count INTEGER NOT NULL,
    imported_rows INTEGER NOT NULL,
    skipped_rows INTEGER NOT NULL,
    notes TEXT NOT NULL DEFAULT ''
);
```

### 1.4 操作与变更审计表: `activity`
```sql
CREATE TABLE activity (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    part_id INTEGER,
    action TEXT NOT NULL,                      -- 如 'import', 'update', 'insert', 'status_change'
    title TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    operator TEXT NOT NULL DEFAULT 'AI Import Agent',
    created_at TEXT NOT NULL,
    FOREIGN KEY (part_id) REFERENCES parts(id) ON DELETE SET NULL
);
```

---

## 2. Excel 表格解析与表头嗅探机制 (Header Detection)

由于工程 Excel 表格格式繁多，表头往往**不固定在第 1 行**（常见第 1-3 行为标题横幅、统计汇总、机型范围或说明），AI/脚本必须采用**动态表头嗅探算法**：

1. **工作表过滤**：
   - 跳过名称包含 `Cover`, `Summary`, `封面`, `目录`, `说明`, `模板`, `Template` 的工作表，除非该表包含有效器件数据行。
   - 跳过总行数小于 2 行的空工作表。
2. **表头行探测 (Header Sniffing)**：
   - 逐行检查前 10 行。
   - 计算该行单元格与以下“特征关键字”的命中数：
     `BHS`, `零件号`, `Sachnummer`, `Material No`, `Part No`, `Description`, `中文名称`, `英文描述`, `型号`, `Type`, `Specification`, `品牌`, `Manufacturer`, `订货号`, `Order No`。
   - 命中特征关键字 $\ge 2$ 且包含至少一个零件号/名称列的行，判定为**有效表头行 (Header Row)**。
   - 表头行之后的非空行全部视为元器件数据行。
3. **有效数据行判定**：
   - 提取的 `bhs_no`、`description_cn`、`description_en`、`spec` 四者不能全部为空。
   - 排除整行合并的章节提示行（如 `=== 动力部分 ===`）。

---

## 3. 字段映射矩阵 (Field Mapping Matrix)

AI/脚本解析表头名称时，必须忽略前后空格、换行符、下划线及大小写，并按照以下映射规则归一化到对应字段：

| 目标字段 (`parts`) | 来源列关键词匹配规则 (中 / 英 / 德) | 备选/容错处理 |
| :--- | :--- | :--- |
| `bhs_no` | `BHS No`, `BHS-Nr`, `BHS零件号`, `零件号`, `物料号`, `Material No`, `Sachnummer`, `Part No`, `BHS Code` | 必须清洗非数字字母前后缀 |
| `description_cn` | `中文描述`, `中文名称`, `元件名称`, `品名`, `名称`, `Description (CN)`, `Chinese Name` | 优先取含中文的描述列 |
| `description_en` | `英文描述`, `英文名称`, `Description (EN)`, `Description`, `Bezeichnung`, `English Name` | 若仅一列 Description 且含英文则落此列 |
| `spec` | `型号`, `规格`, `型号规格`, `规格型号`, `Type`, `Specification`, `Dimensions`, `Tech Spec` | 保持原始参数格式 |
| `manufacturer` | `品牌`, `厂家`, `供应商`, `生产商`, `Manufacturer`, `Hersteller`, `Supplier`, `Brand` | 规范化常见厂商名称 |
| `order_no` | `订货号`, `订购号`, `采购代码`, `Order No`, `Order Code`, `Bestell-Nr`, `Part Code` | 保持文本格式避免科学计数法 |
| `unit` | `单位`, `计量单位`, `Unit`, `ME`, `Mengeneinheit` | 缺省填写 `"个"` 或 `"PC"` |
| `dispo` | `Dispo`, `属性`, `MRP`, `类别`, `Dispo-Gruppe` | 如 "E", "M7", "R" 等 |
| `cert` | `认证`, `认证标准`, `UL`, `CE`, `CCC`, `Cert`, `Certification` | 多项合并以逗号分隔 |
| `machine` | `机型`, `适用机型`, `功能段`, `Machine`, `Machine Type`, `Usage` | 如 "SF", "DF", "通用" 等 |
| `import_flag` | `WHR`, `进口`, `进口标识`, `Import`, `WHR/进口` | 如 "WHR", "进口", "国产" |
| `remark` | `备注`, `说明`, `Remark`, `Comment`, `Notice`, `Anmerkung` | 保留设计人员原始说明 |
| `price` | `单价`, `价格`, `Price`, `Cost`, `Unit Price` | 保留货币符号或数值 |
| `spare_level` | `备件级别`, `备件等级`, `Spare Level`, `Spare Part Level` | 如 "A", "B", "C" 等 |
| `slob` | `SLOB`, `呆滞`, `死库存`, `Obsolete Stock` | 标识物料健康度 |
| `safety_stock` | `安全库存`, `Safety Stock`, `Min Stock` | 文本化存储 |
| `protocol_stock`| `协议库存`, `协议量`, `Protocol Stock` | 文本化存储 |
| `stock` | `当前库存`, `库存`, `Stock`, `Inventory`, `Bestand` | 文本化存储 |
| `moq` | `起订量`, `最小包装`, `MOQ`, `Min Order Qty` | 文本化存储 |
| `delivery` | `货期`, `供货周期`, `交期`, `Lead Time`, `Delivery Time` | 如 "4 weeks", "现货" 等 |
| `alternate_no` | `替代件号`, `旧零件号`, `G No`, `G-No`, `Old Part No`, `Alternate No` | 辅助历史查找 |
| `cn_part_no` | `CN零件号`, `CN件号`, `CN Part No`, `CN Code` | 中国专用件号 |
| `dongan_part_no`| `东安零件号`, `DongAn Part No`, `东安号` | 外协厂专用件号 |
| `s_part_no` | `S零件号`, `S件号`, `S Part No` | S 特殊件号 |
| `status` | 包含“停用”、“作废”、“obsolete”、“inactive”等字样 | 映射为 `'inactive'` 或 `'obsolete'`，默认 `'active'` |

> **未映射列处理**：若 Excel 表格中含有特殊的业务列（如特定的测试参数、电气寿命、接线针脚定义等），不得随意丢弃，必须序列化为 JSON 字符串存入 `extra_json` 列。

---

## 4. 数据清洗与规范化要求 (Data Sanitization)

AI/脚本处理各单元格数据时，必须执行以下清洗管道：

1. **去除幽灵字符**：清除 Excel 中常见的 `\r`, `\n`, `\t`, `\u200b` (零宽空格), `\xa0` (非断行空格)。多个连续空格收敛为单个半角空格。
2. **全角转半角**：将全角数字 `１２３`、全角标点 `，`、全角英文字符 `ＡＢＣ` 统一转为标准 ASCII 半角字符。
3. **数字精度与科学计数法防损**：
   - BHS 零件号与订货号严禁转换为浮点数以防长数字被截断（如 `100917800.0` 或 `1.00918E+08`）。必须提取纯文本字符串。
4. **分类推断 (Category Extraction)**：
   - 若来源文件名符合 `Master E-list_<Category>.xlsx`（例如 `Master E-list_Circuit Breaker.xlsx`），自动提取 `<Category>` 作为元器件的 `category`。
   - 子分类 `subcategory` 默认为当前工作表名 (`source_sheet`)。

---

## 5. 增量导入去重与冲突合并策略 (Deduplication & Merge)

在向现有数据库导入更新或追加新表格时，必须严格执行防误删与防覆盖策略：

### 5.1 唯一键判定
元器件在工程中同一 BHS 编号可能因用于不同机型/不同工作表而出现在不同行。
- 逻辑唯一标识为三元组：`(bhs_no, source_file, source_sheet)`。

### 5.2 冲突处理规则
1. **人工维护保护原则 (Manual Edit Preservation - 最高优先级)**：
   - 检查目标数据库中是否存在同 `id` 或同三元组的记录且其 `origin = 'manual'`。
   - **规则：凡标记为 `'manual'` 的记录，AI 批量导入严禁覆盖其任何字段！** 如有冲突，应将新数据作为一条新的 `origin = 'import'` 记录插入，或在 `activity` 中记录冲突提示。
2. **增量更新对比 (Update vs Insert)**：
   - 若存在相同的 `(bhs_no, source_file, source_sheet)`：
     - 比对各核心字段（名称、规格、品牌、订货号等）。
     - 若内容发生变化，执行 `UPDATE`，更新变化的列，并将 `updated_at` 设为当前时间，同时向 `activity` 表记录一条变更日志。
   - 若不存在，执行 `INSERT`，初始 `created_at` 与 `updated_at` 为当前时间，`origin = 'import'`。

---

## 6. 全文检索字段生成规则 (`search_text`)

为保障 MyGo 原生 GUI 中毫秒级模糊搜索与多词检索的极高体验，任何插入或更新 `parts` 表的操作，**必须**自动重新组装 `search_text` 列：

```python
search_tokens = [
    part.bhs_no,
    part.description_cn,
    part.description_en,
    part.spec,
    part.manufacturer,
    part.order_no,
    part.category,
    part.subcategory,
    part.cert,
    part.machine,
    part.remark,
    part.alternate_no,
    part.related_part,
    part.cn_part_no,
    part.dongan_part_no,
    part.s_part_no,
]
# 清洗空值、转小写、以空格拼接
part.search_text = " ".join(t.strip().lower() for t in search_tokens if t and t.strip())
```

---

## 7. 执行与校验核验清单 (Checklist for AI / Scripts)

在完成对 SQLite 数据库的 Excel 写入后，AI 必须运行以下校验 SQL，并在终端或报告中输出核验结果：

```sql
-- 1. 验证总器件数与各状态分布
SELECT status, COUNT(*) FROM parts GROUP BY status;

-- 2. 验证是否存在无 BHS 编号的无效行
SELECT COUNT(*) FROM parts WHERE bhs_no = '';

-- 3. 验证是否有未归属分类的工作表
SELECT COUNT(*) FROM source_sheets WHERE category = '未分类' OR category = '';

-- 4. 验证最近导入批次日志
SELECT * FROM import_batches ORDER BY id DESC LIMIT 1;
```

满足以上标准即可确保：新导入的数据能被 MyGo 客户端无缝展示，左侧分类树自动更新，顶部工作表标签自适应排列，数据手册自动按 BHS 编号完成精准匹配！
