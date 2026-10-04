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

样本原生代码提供 DES/ECB/PKCS5Padding 解密相关参数。已取得并校验的目录包含 606 个节点、31 个省级地区；快照在开发目录 `internal/catalog/servers.json`。APK 内原始模块目录保存在 `internal/catalog/tests.json`。

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
- 原 APK 支持 ICMP 和 TCP fallback。当前 TCP 值为 Go 建连耗时，仅作诊断；主界面 HTTP 延迟是新实现的首字节探测，不宣称等同原 APK Ping。
- 真实节点验证完成会话、下载、上传和释放；部分节点不响应或拒绝申请，不能仅由这些现象推断客户端协议错误。

关键证据类：`SpeedTestTask.java`、`SpeedUpDownTask.java`、`TcpConnection.java`、`PingTask.java`、`HttpUtil.java`。本地反编译目录为 `analysis/decompiled/sources/`。
