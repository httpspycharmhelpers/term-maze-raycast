# term-maze-raycast

Go 版 raycast 风格 3D 迷宫：融合

- [raycast-game](https://github.com/httpspycharmhelpers/raycast-game) — 画面与操作：墙壁 / 天花板 / 地板 / 俯仰 / 视高 / 视野的 DDA 光线投射渲染，按键布局完全一致
- [term-maze-3d](https://github.com/Yadav106/termi-maze-3d) — 迷宫玩法：随机迷宫、出生点 / 出口、到达出口自动生成新迷宫

用 Go 实现，零第三方图像库，仅依赖 `eiannone/keyboard`（按键）与 `x/sys/unix`（终端尺寸）。

## 画面

- 墙壁使用 DDA 光线投射 + 边缘描边：竖直边 `|`、水平边 `_`（俯仰时 `/` `\`）、转角 `·`
- 天花板留空，地板用 `.` 填充，地平线随俯仰 / 视高偏移
- 画面自适应终端尺寸：终端缩小或放大，游戏画面跟着缩放，不会出现乱码

## 操作（与 raycast-game 一致）

| 按键 | 功能 |
| --- | --- |
| 2 / 8 | 前进 / 后退 |
| 4 / 6 | 向左 / 向右平移 |
| 1 / 3 | 向左 / 向右旋转 |
| 5 / 0 | 摄像头往上 / 往下 |
| 7 / 9 | 抬头 / 低头 |
| + / - | 增加 / 减少视野 |
| % | 开关小地图（占屏幕 1/4，左上角） |
| q | 退出 |

## 构建运行

```bash
# Termux
pkg install golang
cd term-maze-raycast
go build -o term-maze-raycast .
./term-maze-raycast
```

迷宫每次启动随机生成（递归回溯 + 少量破墙）；走到出口 `E` 自动换一张新迷宫。小地图中：`#` 墙、`.` 空地、`@` 玩家、`S` 出生点、`E` 出口。