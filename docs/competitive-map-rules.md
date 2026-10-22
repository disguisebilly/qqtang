# 地图变身道具配置

[data/qqt_competitive_map_rules.json](../data/qqt_competitive_map_rules.json)是服务端运行时配置，完整列出326个竞技地图的ID、名称和变身道具开关。可以直接搜索中文地图名定位条目。

```json
{
  "map_id": 211,
  "map_name": "景阳岗",
  "transformation_items_enabled": false
}
```

- `true`：允许原有掉落表里的变身道具；不额外增加该地图原本没有的变身种类。
- `false`：在道具容量分配前过滤变身道具，剩余道具继续按正常逻辑分配。
- 未列出的地图、未填写的开关或缺少配置文件：默认开启。

`map_id`是实际匹配依据，`map_name`供阅读和搜索。普通场与道具场使用独立地图ID。默认关闭211/212/214/216，即景阳岗和新年瑞兽的普通场、道具场版本；其余地图保持开启。

配置在服务端启动时加载。修改文件后重启服务端即可，**不需要重新编译、构建发行包或重新生成配置**。开关只影响地图掉落池，不修改变身效果、携带道具或Boss技能。

默认服务端配置[server-directory-local-ui.json](../configs/server-directory-local-ui.json)通过`competitive_map_rules_path`引用这个文件。路径相对于服务端JSON所在目录解析，默认值为`../data/qqt_competitive_map_rules.json`。正式对局和离线引擎共用地图规则与分配实现，原始地图文件不被改写。

JSON格式错误、重复或不存在的地图ID会报告配置错误。新增客户端地图时，可按相同结构添加条目；没有条目的地图默认开启。
