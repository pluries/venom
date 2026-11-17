---
translates: mqtt
title: MQTT
description: 如何在 venom 测试套件中使用 MQTT executor。
fingerprint: 9ae821f545fded1165c473e275c91dc01e721875681bba4d6c8f7a000da248fd
drafted_by: claude
drafted_on: 2026-11-16
status: machine-drafted
---
# Venom - MQTT Executor {#venom---executor-mqtt}

用于读取和写入 MQTT 主题的步骤。

示例见 `tests/mqtt.yml`

## 输入 {#input}

ClientType 可以是以下之一：
* "publisher"
* "subscriber"
* "persistent_queue"

顾名思义，"persistent_queue" 通过设置 session_clean 属性来创建持久队列，以便之后的 "subscriber" 步骤可以取回数据。"persistent_queue" 与 "persistSubscription" 配对使用，后者可以为 true 或 false，用于请求或释放持久主题订阅。
persistent_queue 与 subscriber 必须使用相同的客户端 ID，这样代理才能跨连接跟踪状态。完成后请记得删除主题注册。
请注意这里使用的名称是 "persistent_queue"，而不是 MQTT 更常用的 clean_session。这样做是为了减少在步骤配置中遗漏 "persistent_queue" 选项时出现意外行为。

## 限制与未来改进 {#limitations-and-future-improvements}

### 限制 {#limitations}

由于 venom 启动每个任务时不在步骤之间保留任何状态，而 mqtt 如果在消息发送之后才订阅就会丢失消息，因此我们需要一种机制，为之后的 mqtt 步骤注册持久主题。这要求测试增加一些额外的步骤，在步骤序列开始前预先注册持久主题订阅，并在序列结束时注销。

MQTT 可能在连接之后、服务来得及绑定订阅处理器之前就投递消息。对于 "subscriber" 步骤，我们在连接之前用 AddRoute 附加处理器，连接之后再订阅。这样可以确保不丢失消息，并让订阅按您的预期工作。使用持久订阅时，需要这样做以避免消息丢失。

### 未来工作 {#future-work}

* 增加从文件而不是 yaml 配置内部获取消息内容的能力。这不必局限于该 executor
* 增加对编解码器的支持，以便支持 json 之外的序列化格式。这本应是任何 executor 都能利用的能力，而不是逐个单独解决
* 为 MQTT 接口增加 TLS 支持
