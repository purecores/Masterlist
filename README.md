# BHS 电气柜单元器件库 (MyGo Native)

基于 [MyGo](https://github.com/egoist/mygo) 原生 GPU 渲染引擎（Direct3D 11 / purego）打造的本地电气柜单元器件库桌面应用。无需 Electron、WebView2 或 Node，运行时为独立原生程序。

![BHS Masterlist 深色工业界面](docs/preview-dark.png)

## 功能

- 全部原表、原表分类、工作表三层浏览，保留 Excel 查表习惯。
- BHS 编号、名称、型号、品牌、订货号模糊搜索，支持常见电气同义词。
- 单件新增、修改，重复 BHS 编号提示与版本保留。
- 数据手册 PDF 按文件名中的 BHS 编号自动关联，支持多个相对或绝对路径。
- 原 Excel 一键打开、文件定位和自定义原表目录。
- Excel 增量检测与同步，手工维护记录不会被批量覆盖。
- 白天浅色与夜晚深色主题切换。
- Windows User 权限便携运行，全部状态限制在程序目录内。

## 快速运行

Release 中下载 `BHS-Masterlist-<version>-windows-amd64.zip`，解压后双击 `BHS-Masterlist.exe`。Release 包包含示例数据库、SQLite 运行库和 `start.cmd`。

命令行参数：

```powershell
.\BHS-Masterlist.exe -db "D:\data\my-masterlist.db"
.\BHS-Masterlist.exe -query "100917800"
.\BHS-Masterlist.exe -settings
```

## 从源码构建

环境要求：Go 1.27.3 或更高版本、Windows 10/11 x64。

```powershell
go mod download
go test ./...
go build -ldflags "-H windowsgui -s -w" -o BHS-Masterlist.exe .
```

若要在开发目录运行，将 `mygo-sqlite3.dll` 放入可执行文件旁；程序也会从内嵌资源释放该运行库。生成的 `data/masterlist.db` 是脱敏示例库，真实业务数据不会被提交到源码仓库。

## 便携目录

```text
Portable/
├── BHS-Masterlist.exe
├── mygo-sqlite3.dll
├── config.json
├── icon.png
├── README.md
├── AI_EXCEL_IMPORT_RULES.md
├── start.cmd
└── data/
    └── masterlist.db
```

- `config.json` 用于配置数据手册和原表路径，首次运行可留空或使用相对路径。
- 程序不写注册表、不写 `%APPDATA%`，窗口状态和配置均保存在程序目录。

## 数据说明

仓库和 Release 仅提供演示用种子数据库，其中不含生产零件数据。需要导入业务 Excel 时，参考 [AI_EXCEL_IMPORT_RULES.md](AI_EXCEL_IMPORT_RULES.md)。

## 技术栈

MyGo `v0.7.3`、Direct3D 11、purego、SQLite、excelize、Go embed。
