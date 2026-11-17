# BHS 电气柜单元器件库 (MyGo Native)

基于 [MyGo](https://github.com/egoist/mygo) 原生 GPU 渲染引擎（Direct3D 11 / purego）打造的本地电气柜单元器件库桌面应用。无需 Electron、WebView2 或 Node，运行时为独立原生程序。

![BHS Masterlist 深色工业界面](docs/preview-dark.png)

## 功能

- 全部原表、原表分类、工作表三层浏览，保留 Excel 查表习惯；工作表用可搜索的选择器切换，不再遮挡原表内容。
- BHS 编号、名称、型号、品牌、订货号模糊搜索，支持常见电气同义词。
- 器件列表排序（原表顺序、BHS 编号、元件名称、品牌、型号、最近修改，可升/降序）与筛选（状态、类别、品牌、是否有数据手册），筛选条件可一键清除。
- 单件新增、修改，重复 BHS 编号提示与版本保留。
- 数据手册 PDF 按文件名中的 BHS 编号自动关联；可选常驻 Windows 状态栏（托盘图标），关闭窗口可隐藏到状态栏。
- 数据手册根目录**递归检索子文件夹**，支持 UNC 网络盘与映射盘（如 `Z:\手册`、`\\server\share\pdf`）。
- 同一物料可关联多份手册并逐一查看；文件名相同且大小相同的文件被视为同一份，只显示一次。
- 手册**文件列表**窗口：按文件夹分组浏览、搜索、定位、打开，并提供**手动同步**：只同步用户主动触发的变更，网络波动或共享盘掉线时自动沿用上次索引。
- 索引持久化到 `data/pdf-index.json`，第二次启动直接读取，不重复遍历网络盘；可设置索引过期时间或手动重建。
- 可选择**只读方式打开 PDF**（设置中开启）：打开的是带“只读”标记的私有副本，避免误编辑手册，也不会锁定网络盘上的原文件。
- 原 Excel 一键打开、文件定位和自定义原表目录；Excel 增量检测与同步，手工维护记录不会被批量覆盖。
- 白天浅色与夜晚深色主题切换。
- Windows User 权限便携运行，全部状态限制在程序目录内。

## 快速运行

Release 中下载 `BHS Masterlist-<version>-windows-amd64.zip`，解压后双击 `BHS Masterlist.exe`（或运行 `start.cmd`）。Release 包包含示例数据库、SQLite 运行库和 `icon.png`。

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
go tool mygo build -skip-build-command -platform windows/amd64
```

产物在 `build/windows-amd64/`：带图标与版本信息的 `BHS Masterlist.exe`，以及当前用户安装包 `bhs-masterlist-setup-<version>-amd64.exe`。

若要在开发目录 `go run .`，将 `assets/mygo-sqlite3.dll` 复制到可执行文件旁；程序也会从内嵌资源释放该运行库。生成的 `data/masterlist.db` 是脱敏示例库，真实业务数据不会被提交到源码仓库。

## 便携目录

```text
Portable/
├── BHS Masterlist.exe
├── mygo-sqlite3.dll
├── icon.png
├── config.json            # 首次运行后生成
├── README.md
├── start.cmd
└── data/
    ├── masterlist.db
    └── pdf-index.json     # 手册索引缓存
```

- `config.json` 保存手册/原表目录、主题、状态栏、只读打开、排序与筛选等设置。
- 程序不写注册表、不写 `%APPDATA%`，窗口状态和配置均保存在程序目录。

## 数据说明

仓库和 Release 仅提供演示用种子数据库，其中不含生产零件数据。需要导入业务 Excel 时，参考 [AI_EXCEL_IMPORT_RULES.md](AI_EXCEL_IMPORT_RULES.md)。

## 技术栈

MyGo `v0.7.3`、Direct3D 11、purego、SQLite、excelize、Go embed。
