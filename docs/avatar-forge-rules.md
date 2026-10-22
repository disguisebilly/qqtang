# 装扮锻造配置

锻造配置分为两部分：

| 文件 | 内容 |
| --- | --- |
| `runtime/client-patched/config/avatarforge.ini` | 可锻造物品名单、部位、效果、颜色、拆分与还原材料 |
| [data/qqt_forge_rules.json](../data/qqt_forge_rules.json) | 服务端水晶成功率、特效等级、颜色权重与黑色概率 |

客户端和服务端使用同一份`avatarforge.ini`。界面只列出当前背包中有效且命中名单的物品；加入名单不会自动赠送物品。

允许名单共1329项：帽子478、身体401、翅膀/背饰147、糖泡303。翅膀名单覆盖当前目录全部146种`fpack`，并保留一个旧表ID。资源类别、道具ID和资源ID各有独立含义，不能互相替代。

## 修改可锻造范围

发行包直接修改`runtime/client-patched/config/avatarforge.ini`，然后重启客户端与服务端，无需重新编译。编辑时保留INI原有部位编号、`typename`、`count`与`item.N`格式，以及CP936编码。

源码仓库中的发行默认文件位于[build-assets/client-config/avatarforge.ini](https://github.com/kuuhaku1314/qqtang/blob/main/build-assets/client-config/avatarforge.ini)。构建脚本在复制用户提供的原始客户端后，使用该文件覆盖发行包里的同名配置，因此不会重新带入原始客户端的旧名单。

## 水晶与结果

服务端的水晶规则使用0到100的`success_percent`、等级相对权重`level_weights`、颜色相对权重`color_weights`与`black_percent`。普通颜色支持`blue/red/green/purple`；客户端部分橙色外观对应`red`索引。

- 普通锻造成功时按规则抽取等级和颜色；失败消耗晶石但保持原效果与颜色。
- 已有特效不会因重锻抽到更低等级而降级，颜色独立重抽。
- 拆分清除颜色并保留特效。
- 还原同时清除颜色和特效。

修改服务端概率配置后重启服务端生效。七种水晶都需要完整有效的规则；具体概率以随包JSON为准。

## AI糖泡外观

普通竞技的虚拟AI每局从配置允许的糖泡中随机装备一种，同局优先使用不同款式，并按配置随机搭配锻造颜色和特效等级。款式在允许列表、规范目录与锻造名单的交集中等概率选择；颜色和等级分别等概率选择。外观整局保持固定，下局重新选择。这些是临时外观，不写入玩家背包，也不影响糖泡威力、爆炸范围或AI决策。

可在`configs/server-directory-local-ui.json`现有的`competitive_ai`对象中修改`bubble_costume_item_ids`数组，指定允许AI使用的糖泡装扮。数字是物品ID，不是资源ID；完整295种名称可查询[糖泡ID对照表](ai-bubble-costumes.md)。

在现有`competitive_ai`对象中配置三个数组，例如：

```json
{
  "bubble_costume_item_ids": [119, 256, 257],
  "bubble_costume_color_ids": [0, 2, 3],
  "bubble_costume_effect_levels": [0, 3, 5]
}
```

| 数组 | 取值 | 空数组`[]` |
| --- | --- | --- |
| `bubble_costume_item_ids` | 特殊糖泡物品ID；0表示普通默认泡 | 使用普通默认泡，不附加颜色或特效 |
| `bubble_costume_color_ids` | 0无染色、1黑、2蓝、3红、4绿、5紫 | 不染色，也不附加特效 |
| `bubble_costume_effect_levels` | 0无特效，1～5为特效等级 | 保留所选颜色，不附加特效 |

三个数组中的0都作为随机项参与抽取：糖泡0为普通默认泡，颜色0为无染色，等级0为无特效。**特效只用于有锻造颜色的特殊糖泡**：糖泡0或颜色0被选中时自动抑制特效，无需限制另一个数组。任一数组未填写、为`null`或为`[]`时，该项不启用；没有颜色时也不附加特效。**发布默认配置的三个数组均为`[0]`**，即普通默认泡、无颜色、无特效，可按对照表自行启用其他选项。重复、未知、非糖泡ID或范围以外的颜色/等级会报告配置错误。保存后重启服务端生效，无需重新编译。
