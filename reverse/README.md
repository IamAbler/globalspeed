# GlobalSpeed 协议分析

本目录保存样本分析方法和复现脚本。应用开发与使用说明见根目录 README。

## 目录

```text
scripts/   静态提取、加载器恢复、密钥仿真与 DEX 解密脚本
samples/   本地 APK 样本（不提交）
analysis/  本地分析输出与反编译结果（不提交）
.tools/    本地 Python 仿真依赖及辅助工具（不提交）
```

生成文件、完整反编译源码、二进制样本和工具缓存均不上传 Git。已有完整本地记录位于 `analysis/REPORT.md`。

## 样本

- 文件：`samples/globalspeed_4.4.8_safe.apk`
- 大小：9,229,480 字节
- SHA-256：`97eec1440176cbc08f6eb6327a5fc745b8ba69b927835a9dad1be7d5459aeac8`
- Android 包名：`com.cnspeedtest.globalspeed`
- 版本：4.4.8，版本码 40408

包名是服务协议参数，保留它不影响 GlobalSpeed 的产品名称。

## 方法与复现

从 ZIP、Manifest 与 XML 配置提取开始，恢复 Jiagu 内部 ARM64 加载器，使用 Unicorn 离线仿真密钥生成，再解密业务 DEX。未启动 Android 样本。

在项目根目录依次运行：

```bash
python3 reverse/scripts/analyze_apk.py
python3 reverse/scripts/install_reverse_tools.py
python3 reverse/scripts/unpack_layers.py
python3 reverse/scripts/emulate_loader.py
python3 reverse/scripts/decrypt_business.py
```

工具安装脚本下载适用于 Linux x86_64 的 Unicorn/Capstone wheel。解密恢复三个 DEX，大小分别为 6,316,840、102,148、93,504 字节；长度及 Adler-32 正确，头部 SHA-1 不匹配，保留原始恢复结果。JADX 输出曾出现 15 个反编译错误，不能视为完整可编译源码。

## 节点目录

配置索引：`https://dlcv2.cnspeedtest.cn:8443/TaierAndroid/Config/ConfigMD5.php`。

加密目录：`https://down.cnspeedtest.cn:8043/TaierAndroid/Config/serverlist_encrypt.json`。

样本原生代码提供 DES/ECB/PKCS5Padding 解密相关参数。已取得并校验的目录包含 606 个节点、31 个省级地区；内置快照在 `internal/catalog/serverlist_encrypt.json`，保存厂商原始加密文件。APK 内原始模块目录保存在 `internal/catalog/tests.json`。

## 原版协议

1. GET `/speed/dovalid`，提供 key、flag、bandwidth、model、imei、time、app、token、pkg。
2. 接受响应为 `1-<key>`；空响应、0、2、-1 前缀分别对应原 APK 状态 131–134。
3. GET `/speed/File(1G).dl?r=<秒级时间戳>&key=<key>` 下载。
4. POST `/speed/doAnalsLoad.do` 上传，Key 请求头携带会话，multipart 字段为 upload，boundary 为 `00content0boundary00`。
5. POST `/speed/dovalid?key=<key>` 释放。

会话申请异常时原 APK 最多尝试 3 次、每次设置 3 秒连接和读取超时。Go 使用每次 3 秒总超时，因此不是所有时序细节的逐字节复制。

Token 按 UTF-8 和小写十六进制 MD5 计算：

```text
a = MD5("model=" + model + "&imei=" + identity)
b = MD5("stime=" + timestamp + "&band=" + bandwidth + "&rand=12345555")
token = MD5(a + b)
```

算法来自 `libGSCore.so` 的恢复代码，并通过 ARM64 仿真与 Go 测试向量核验。客户端生成随机 TS 标识，不读取实际 IMEI，不提交原版测试日志或报告。

## 兼容性与差异

- 下载对请求头敏感，使用原版 Chrome UA 与 Accept，禁用自动压缩；不添加 Cache-Control 或 Range。
- 上传使用原版 Dalvik UA、Key、Cache-Control、Charset 和 multipart boundary。原版声明 900000000 字节并按时长断开；Go 使用有限长度 multipart，统计已确认负载。
- Ping 已迁移原版策略：快速模式 10 包、200 ms 间隔，否则 5 包；64 字节、3 秒超时。优先 ICMP，平均值低于 0.1 ms 时回退 TCP；TCP 5 次、50 ms 间隔、整数毫秒均值及整数抖动（差值和除以成功数）。ICMP 抖动按相邻成功回复差值和除以成功数减一。
- `PingTask` 的 TCP packageLost 使用整数除法计算成功数/总数百分比，不能当作真实丢包率。JSON 的 `apkPackageLost` 保留这个兼容字段，`packetLossPct` 另行提供实际失败比例。
- 使用系统 ping 跨平台适配参数和文本解析；Windows 原生命令的整数毫秒输出不能恢复亚毫秒精度，`time<1ms` 作为低于阈值处理并回退。APK 的 Android HttpURLConnection 与 Go TCP 建连实现不同，不能宣称系统栈、默认请求头和时序完全相同。
- 真实节点验证完成会话、下载、上传和释放；部分节点不响应或拒绝申请，不能仅由这些现象推断客户端协议错误。

关键证据类：`SpeedTestTask.java`、`SpeedUpDownTask.java`、`TcpConnection.java`、`PingTask.java`、`HttpUtil.java`。本地反编译目录为 `analysis/decompiled/sources/`。

## 自动选点

`ServerMatchTask` 请求 `https://dlcv2.cnspeedtest.cn:8443/dataServer/mobilematch_many.php`，参数为 ip、network、province、city、wifioper、mobileoperid、ipv6、model、pkg。Android networkCatg==2 时发送 4，否则发送 5。桌面默认发送 5；缺失位置和手机信息留空，目前为 IPv4 模式。

服务返回有序数组，字段包括 hostid、pname、city、port、hostname、hostip。按原顺序对每项运行 `MyPingHelper.do_tcpping(...,2,port)`：超时 1 秒、两次 TCP 建连，取第一个 status==0 且平均值>0.1 ms 的候选。全部失败仍返回第一项，既不按 RTT 排序，也不另加会话验证。

原版网络定位来自 Android IP/位置/SIM 信息。当前未迁移这部分设备权限；默认留空由服务结合请求来源匹配，可提供 CLI 参数或桌面节点筛选作为提示。候选节点地址需为有效公网 IP 和端口。

已验证统计、中文/英文 Windows 响应解析、自动选点顺序、全失败回退和取消。实际南京电信 ICMP 测量：10/10 响应、平均 16.353 ms、抖动 1.7667 ms，并完成上下行各 512 KiB 与会话释放；选点服务亦完成真实查询。系统 ping 的 Windows/macOS 运行仍需对应平台验证。

## 错误状态与交互

错误文案依据 `ExceptionConst.getName`，交互依据 `SpeedTestFragment`：

| 状态 | 提示 |
| --- | --- |
| 121 / 122 | 云服务器连接失败 / 云服务器响应错误 |
| 130 / 131 | 测速服务器连接失败 / 测速服务器无响应 |
| 132 / 133 | 测速服务器繁忙,请稍后重试 / 测速服务器忙，请稍后再试 |
| 134 | 请求排队参数错误 |
| 135 / 136 | 测速服务器或网络异常 / 参数设置错误 |
| 137 / 138 | 下行测试无响应 / 上行测试无响应 |
| 139 | 当前未选择测速服务器 |
| 990 / 992 / 999 | 用户中止测试 / 正在测试中 / 服务器或网络异常 |

桌面测速失败更新状态并短暂显示提示；服务器匹配失败显示 `服务器匹配失败(原因).`，打开“匹配服务器失败，是否重试?”对话框，提供重试和取消。取消也使用原版 990 文案。事件携带结构化 code/message，底层原因保留于 Go 错误链，不把请求 URL、token 或 key 显示给用户。

`SpeedTestTask.run` 仅记录 dequeue 返回值，不覆盖测速状态。Go 同样保留成功结果，释放失败时 `released=false`，界面不声称会话已释放。Go 的压缩响应检查、有限流量预算及 HTTP 实现仍可能产生与 Android 不同的失败条件；这些检查的提示映射到相应状态，不能视为完整 Android 网络栈仿真。

## 桌面图标

用户指定复用样本图标。Manifest 的 application icon 为 `0x7f08016c`，`R.drawable.taierspeed_logo` 对应 `res/drawable-hdpi-v4/taierspeed_logo.png`。原始 512×512 PNG 原样提取到 `build/appicon.png`，未重绘或删除图片内文字。Windows/macOS 由 Wails 转换并打包；Linux 嵌入原 PNG 并提供启动器文件。

## 目录密钥与更新复现

密钥本地固定在 `libGSCore.so`，不是云端申请。`Jni.e()` 位于 `0x7aa5c`，读取全局 `0x10d028` 的指针，指向 `0xd90b3` 的字节串 `dw!@#$%^`，转换成反转的十六进制表示；Java `DesUtil.constDecrypt` 还原八字节 DES key。目录是十六进制文本，DES/ECB/PKCS5Padding，解密后 `DesUtil.unPkcsPadding` 还会去除 1–7 的内层填充，8 则保留。Android JSONTokener 接受末尾 ASCII 控制空白；Go 解密也处理这一兼容细节。

云端索引的 `serverlist_encrypt_url` 给出 filename 下载地址和密文文件 MD5，没有目录密钥字段。`SettingsHelper.updatefile_fromserver` 先比较本地密文 MD5，再下载临时文件、校验并替换。Go 保留原始下载字节、校验 MD5、解密验证目录后用临时文件替换；使用默认配置目录中的 `serverlist.json`，不写解密后的目录文件。更新仅接受 HTTPS 厂商域名，不采用索引中的其他设备配置。

内置快照仍为 MD5 `6d492c0d5d07e790ac8e609097879b74`、606 节点。2026-10-05 实测更新取得 604 节点，证明运行目录使用在线数据而非固定快照。CLI `update` 手动更新，`nodes --path` 显示路径，`nodes/speed --no-update` 跳过更新检查。
