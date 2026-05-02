# DogOffice 存檔 API spec（v2 / v3 接案制）

本文件描述 `dog-company` 後端**重寫**後的存檔端點。極簡版：一人一份 current save，不做歷代封存。

> 規格版本：draft-10（2026-05-02，新增 v3：工具系統 + 累積進度欄位正式入錄）
> 基準 API base：`https://dog-company-production.up.railway.app/api/v1`

> ⚠️ **v1 永久停用**。v2 與 v3 同時支援；前端目前 `SAVE_VERSION = 3`，但伺服器仍須接受 v2 payload 以相容舊雲端紀錄。
> v3 是 v2 的純擴充：新增 optional 欄位、`Dog.stats` 加上 `patience` 第五維、`companyBuffs` 補若干 boost 欄位。**沒有 v2 ↔ v3 轉換邏輯**，server 收到什麼就存什麼，client 自己處理 fallback。

---

## 總覽

| Method | Path | Auth | 說明 |
|--------|------|------|------|
| GET    | `/saves` | Bearer | 取得目前使用者的 current save |
| POST   | `/saves` | Bearer | 新增 / 覆寫 current save |
| DELETE | `/saves` | Bearer | 刪除 current save |

**認證**：`Authorization: Bearer <access_token>`，與 `/auth/me` 相同。

**Response 格式**：沿用 `CommonResponse`：`{ code, message, data }`。

---

## 資料模型

### SavePayload（POST /saves 的 body）

```json
{
  "version": 2,
  "revision": 3,
  "data": {
    "day": 42,
    "money": 1240,
    "reputation": 55,
    "tierBudget": 78,
    "companyBuffs": {
      "speedBoost": 2,
      "qualityBoost": 1,
      "teamworkBoost": 1,
      "charismaBoost": 0,
      "decor": 3
    },
    "officeLevel": 2,
    "purchases": { "snack": 3, "desk": 1, "coffee": 1 },

    "clients": [ /* Project[] */ ],
    "projectsCompleted": 12,
    "projectsFailed": 2,
    "lastRerollDay": 38,

    "vacancy": false,
    "vacancyTimer": 0,
    "bankrupt": false,
    "bankruptCountdown": 0,
    "tutorialStep": 7,
    "recruitmentClosed": false,

    "staff": [ /* Dog[] */ ],
    "log": [ /* LogEntry[]，最多 10 筆 */ ],

    "ipoAchievedAt": null,
    "ipoDismissed": false,

    "loanTaken": false,
    "loanRepayDaysLeft": 0
  }
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `version` | int | 存檔格式版本。**接受 `2` 或 `3`**。`version=1` 會被前端視為無效；`version` > 3 後端應回 400 / 4001 |
| `revision` | int, **optional** | 客端持有的存檔 revision。衝突偵測用。首次存檔可省略，server 從 1 起算 |
| `data` | object | 實際遊戲狀態。下面詳列 |

---

### GameSaveData 欄位（v2 base + v3 增補）

> v3 新增的欄位在下表會以「v3」標註；v2 不含 / 應 ignore。所有 v3 欄位在 v2 client 都不會出現，所以全部 optional。

#### 公司基本狀態

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `companyName` | string, optional | 0-8 字元 | **v3**：玩家自訂公司名。空字串代表尚未命名（玩家下次按「開始經營」會被導向命名 modal） |
| `day` | int | ≥1 | 遊戲第幾天 |
| `money` | int | ≥0 | 資金 |
| `reputation` | number | 0-100 | **信譽**（v2 新欄位，取代舊 health；可為小數，如 +0.5）|
| `tierBudget` | int | ≥0 | 案件稀有度預算（每天 morning 會由 client 重算）|
| `companyBuffs` | object | — | 公司全域 buff（見下） |
| `officeLevel` | int | 0-4 | 辦公室等級（容量 / IPO 條件用）|
| `officeSkin` | int, optional | 0-officeLevel | **v3**：辦公室視覺造型（玩家可在已解鎖等級之間切換）；缺此欄位視為 `officeLevel` |
| `purchases` | object | — | 商品 id → 購買次數（=等級，最高 5），如 `{ "snack": 3 }` |

#### `companyBuffs` 子物件

| key | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `speedBoost` | number | ≥0 | 來源：desk / coffee / gym |
| `qualityBoost` | number | ≥0 | 來源：policy |
| `teamworkBoost` | number | ≥0 | 來源：sofa / gym |
| `charismaBoost` | number | ≥0 | 預留 |
| `decor` | number | ≥0 | 來源：lamp，影響 walker 視覺 |
| `categorySpeed` | object, optional | 4 個 ProjectCategory key → number ≥0 | **v3**：分類別速度加成（tech/design/marketing/service） |
| `categoryQuality` | object, optional | 4 個 ProjectCategory key → number ≥0 | **v3**：分類別品質加成 |
| `patienceBoost` | number, optional | ≥0 | **v3**：耐心 buff |
| `fatigueRecoveryBonus` | number, optional | ≥0 | **v3**：疲勞回復加成 |

#### 接案系統（v2 新增）

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `clients` | array of Project | — | 案件陣列（offered + active + 最近結算 done/failed 共 ≤30 筆）|
| `projectsCompleted` | int | ≥0 | 累計完成案件數（IPO 條件之一：≥30）|
| `projectsFailed` | int | ≥0 | 累計失敗案件數 |
| `lastRerollDay` | int | ≥0 | 最近重 roll 收件匣的天數，0 = 沒重 roll 過 |

#### 招募 / 系統旗標

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `vacancy` | bool | — | 是否進入「人才荒」空窗期 |
| `vacancyTimer` | int | ≥0 | 空窗剩餘天數 |
| `bankrupt` | bool | — | 是否破產 |
| `bankruptCountdown` | int | 0-5 | **連續資金 ≤0 的天數**（達 5 → 破產）|
| `tutorialStep` | int | 0-7 | 教學進度。≥7 表示已看完 |
| `recruitmentClosed` | bool | — | 玩家手動暫停招募 |

#### 集合

| 欄位 | 型別 | 說明 |
|---|---|---|
| `staff` | array of Dog | 員工陣列 |
| `log` | array of LogEntry | 最近 10 筆日誌（`{ day, msg }`）|

#### IPO 勝利

| 欄位 | 型別 | 說明 |
|---|---|---|
| `ipoAchievedAt` | int \| null | IPO 達成天數，未達成 = `null` |
| `ipoDismissed` | bool | 玩家是否已關閉 IPO 慶祝 modal |

#### 銀行貸款（一次性救急機制）

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `loanTaken` | bool | — | 是否已借過。**一輩子限一次**，true 後永久不可再觸發貸款 modal |
| `loanRepayDaysLeft` | int | 0-80 | 剩餘還款天數，0 = 無貸款。每天 morning 自動扣 $5，扣到 0 為止 |

#### v3 新增 top-level 欄位

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `tools` | array of Tool, optional | 長度 ≤ 100 | **v3**：玩家庫存的工具陣列（見下節 Tool 結構）|
| `teams` | object, optional | 4 key (tech/design/marketing/service) → Team | **v3**：分類別團隊狀態 |
| `specialTasks` | object, optional | key 為 officeLevel 數字字串 (`"0"`–`"4"`) → SpecialTask | **v3**：特殊任務進度，前端會在缺欄位時依 officeLevel 重建 |
| `unlockedAchievementIds` | array of string, optional | 長度 ≤ 50 | **v3**：已解鎖成就 id 列表 |
| `claimedStarterPack` | bool, optional | — | **v3**：本局是否已領新手禮包 |

#### Tool 子物件結構（v3）

```json
{
  "instanceId": "tool_a1b2",
  "defId": "tech-keyboard",
  "name": "機械鍵盤",
  "iconName": "toolKeyboard",
  "category": "tech",
  "grade": "B",
  "speedBoost": 0.8,
  "qualityBoost": 0.6,
  "traits": ["fastStart"],
  "obtainedDay": 14
}
```

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `instanceId` | string | — | 唯一識別，員工 `equippedToolId` 引用此值 |
| `defId` | string | — | 工具定義 id（前端常數），server 不驗列舉 |
| `name` | string | — | 顯示名（前端 cache 用，避免日後 def 改名讓老存檔顯示異常）|
| `iconName` | string | — | 圖示 key |
| `category` | string | `tech` / `design` / `marketing` / `service` | 對應的 ProjectCategory |
| `grade` | string | `S` / `A` / `B` | 工具等級 |
| `speedBoost` | number | 0–5 | 速度加成 |
| `qualityBoost` | number | 0–5 | 品質加成 |
| `traits` | array of string | 長度 ≤ 4 | trait id 列表，可能為空 |
| `obtainedDay` | int | ≥ 1 | 取得當天的 day 編號 |

#### Dog v2 子物件結構

```json
{
  "id": "dog_42",
  "role": "工程師",
  "breed": "柯基",
  "emoji": "🐕",
  "name": "旺財",
  "traits": ["低調"],
  "flavor": "...",
  "passive": "...",
  "motto": "...",
  "stats": { "speed": 9, "quality": 7, "patience": 5, "teamwork": 4, "charisma": 3 },
  "grade": "B",
  "expectedSalary": 22,
  "severance": 66,
  "patience": 5,
  "score": 0,
  "image": "/assets/dog-profiles/engineer-1.png",
  "isCEO": false,
  "interview": { "q": "為什麼想加入？", "goodAnswer": "想做事", "badAnswer": "錢多" },

  "fatigue": 12,
  "loyalty": 55,
  "experience": 18,
  "assignedProjectId": "p_8",
  "daysAtCompany": 14,
  "unhappyLeaveDays": 0,
  "onLeaveDay": null,
  "learnedTraits": ["overtime", "mentor"],
  "pendingTraitChoice": null,

  "level": 3,
  "rosterId": "engineer_corgi_1",
  "fragments": 2,

  "status": "active",
  "pipDaysLeft": 0,
  "pipScore": 0,
  "pipTasks": [],

  "equippedToolId": "tool_a1b2"
}
```

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `id` | string | — | 唯一識別 |
| `stats` | object | speed/quality/teamwork/charisma 各 1-10；v3 加 `patience` 1-10 | v2 為 4 維；v3 起 `Dog.stats.patience` 從 top-level 移入。後端校驗時對 `patience` 缺值不擋 |
| `patience` | int, optional | 1-10 | top-level 欄位，與 `stats.patience` 同義，初期遷移殘留；後端不要求一致 |
| `score` | int | — | 面試評分快取 |
| `image` | string | — | 大頭貼路徑（前端常數產出，後端不驗）|
| `isCEO` | bool, optional | — | CEO 彩蛋角色 |
| `interview` | object, optional | — | 面試 Q&A：`{ q, goodAnswer, badAnswer }`，到職後可能被清掉 |
| `onLeaveDay` | int \| null, optional | — | 准假當天 day 編號，未請假為 `null` |
| `level` | int | 1-10 | **團隊升級系統**：玩家用 $ 或碎片升級。v4+ 必填；v2 舊存檔載入時會 fallback 為 1 |
| `rosterId` | string, optional | — | 對應 `DOG_ROSTER` 條目，圖鑑唯一識別。重抽到同條目 → 加 `fragments` |
| `fragments` | int | ≥0 | 累積碎片，用來升級（升 Lv N→N+1 需要 N 個） |
| `status` | string, optional | `active` / `pip` | PIP 觀察期狀態，缺值視為 `active` |
| `pipDaysLeft` | int, optional | ≥0 | PIP 剩餘天數 |
| `pipScore` | int, optional | — | PIP 期間累計分數，可正可負 |
| `pipTasks` | array, optional | — | PIP 任務清單，每筆 `{ text: string, done: bool }` |
| `equippedToolId` | string \| null, optional | — | **v3**：員工目前裝備的工具 `instanceId`，未裝備為 `null`/`undefined`。可不存在於 `tools[]`（前端會在載入時清成 `null`），server 不需 hard-fail |
| `grade` | string | `S` / `A` / `B` / `C` / `D` | CEO 不算 grade |
| `morale` | int | 0-100 | 個人士氣（**v2 從全公司 morale 改成個人**）|
| `fatigue` | int | 0-100 | 疲勞 |
| `loyalty` | int | 0-100 | 忠誠度（防挖角護盾）|
| `experience` | int | ≥0 | 累積經驗，達門檻自動升 grade |
| `assignedProjectId` | string \| null | — | 目前指派到的案 id |
| `daysAtCompany` | int | ≥0 | 在公司天數（自然累積 loyalty）|
| `unhappyLeaveDays` | int | ≥0 | 連續被拒請假次數（連 3 次自動離職）|
| `onLeaveDay` | int \| null | — | 准假當天的 day 編號；該日該員工 0 貢獻；隔天 runProjectsDay 自動清空 |
| `learnedTraits` | array of string | — | 已習得特性 id 列表（升級時 +1）；舊存檔缺此欄位視為 `[]` |
| `pendingTraitChoice` | object \| null | — | 待選 3 個特性。結構：`{ "choices": [...], "roundsLeft": 1 }`。`roundsLeft > 1` 代表選完還會接下一輪（S 直接面試會給 2 輪） |

**特性發放規則**（前端控制，後端只負責存）：
- 升級 D→C / C→B：不發特性
- 升級 B→A：1 輪選擇（roundsLeft=1）
- 升級 A→S：1 輪選擇
- 直接面試 A 級：1 輪選擇
- 直接面試 S 級：2 輪選擇（roundsLeft=2，第一輪選完自動進第二輪）

**`learnedTraits` 與 `pendingTraitChoice` 的 trait id 列舉**：`overtime` / `perfectionist` / `mentor` / `haggler` / `ironHeart` / `catalyst` / `enduring` / `social`。後端不需驗證 id 是否在列舉內（前端控制），但建議陣列長度 ≤ 8。

#### Project 子物件結構

```json
{
  "id": "p_8",
  "clientName": "貓貓重工",
  "clientTier": 2,
  "category": "tech",
  "title": "API 性能調校",
  "difficulty": 1.0,
  "workRequired": 60,
  "workDone": 24,
  "qualitySum": 168,
  "expectedQuality": 3.0,
  "reward": 240,
  "penalty": 96,
  "defaultDeadlineDays": 5,
  "deadlineDay": 47,
  "graceDays": 2,
  "assignedStaffIds": ["dog_42", "dog_15"],
  "status": "active",
  "reputationDelta": { "success": 3, "fail": -5 },
  "createdDay": 40,
  "acceptedDay": 42,
  "events": [],
  "pendingEvent": null,
  "rewardMul": 1.0,
  "qualityMul": 1.0,
  "eventCount": 0,
  "bonusEventChance": 0
}
```

| 欄位 | 型別 | 說明 |
|---|---|---|
| `id` | string | 唯一識別 |
| `clientTier` | int | 1-5 |
| `category` | string | `tech` / `design` / `marketing` / `service` |
| `status` | string | `offered` / `active` / `done` / `failed` / `late` |
| `deadlineDay` | int | 絕對 day 數；`offered` 狀態 = `-1` 表示未啟動 |
| `acceptedDay` | int, optional | 玩家接案時的 day；offered 狀態無此欄位 |
| `pendingEvent` | object \| null | 待玩家處理的事件（卡進度）|
| `events` | array | 已觸發過的事件紀錄（最多 1 個）|

`ProjectEvent` 結構：
```json
{
  "kind": "changeRequest",
  "triggeredDay": 45,
  "resolved": false,
  "targetDogId": "dog_42"
}
```

`kind` 列舉：`changeRequest` / `earlyDeliver` / `upsell` / `bugBurst` / `dogLeaveAsk` / `poaching`。

---

### Dog 名冊白名單（v3+，sanity 用）

`Dog.rosterId` 是玩家招募的唯一識別。**只有列在下方的 65 個 id 才算合法**，server 收到 payload 內任何 `Dog.rosterId` 不在這份清單時應回 400 / 4002（`message: "invalid rosterId"`）。

> rosterId **可省略**（傳統候選人 / CEO 彩蛋等舊資料沒有 rosterId 欄位）。只在「有提供」時才驗證。

每筆格式 `rosterId | name | breed | role | industry | grade`：

#### tech 工程（16）

```
tech-D-1 | 橘長     | 柴犬           | 工程師 | tech | D
tech-D-2 | 咬咬     | 哈士奇         | 工程師 | tech | D
tech-D-3 | Bug      | 柯基           | 工程師 | tech | D
tech-D-4 | Steve    | 雪納瑞         | QA     | tech | D
tech-D-5 | Howhow   | 貴賓           | QA     | tech | D
tech-D-6 | 小吉     | 比熊           | QA     | tech | D
tech-C-1 | 魷魚     | 邊境牧羊犬     | 工程師 | tech | C
tech-C-2 | Tony     | 黃金獵犬       | 工程師 | tech | C
tech-C-3 | CC       | 臘腸           | QA     | tech | C
tech-C-4 | 泡泡     | 阿拉斯加       | QA     | tech | C
tech-B-1 | Nancy    | 德國牧羊犬     | 工程師 | tech | B
tech-B-2 | Luke     | 惠比特         | 工程師 | tech | B
tech-B-3 | Vivi     | 杜賓           | QA     | tech | B
tech-A-1 | 小得     | 羅威那         | 工程師 | tech | A
tech-A-2 | 缺缺     | 伯恩山犬       | QA     | tech | A
tech-S-1 | 小蘇     | 大白熊         | 工程師 | tech | S
```

#### design 設計（16）

```
design-D-1 | 塗塗     | 比熊           | 美術   | design | D
design-D-2 | 甜甜圈   | 蝴蝶犬         | 美術   | design | D
design-D-3 | 色色     | 馬爾濟斯       | 美術   | design | D
design-D-4 | 泡芙     | 柴犬           | 企劃   | design | D
design-D-5 | 舒舒     | 柯基           | 企劃   | design | D
design-D-6 | Wish     | 巴哥           | 企劃   | design | D
design-C-1 | Joy      | 貴賓           | 美術   | design | C
design-C-2 | 小麥     | 雪納瑞         | 美術   | design | C
design-C-3 | 阿雅     | 哈士奇         | 企劃   | design | C
design-C-4 | 青青     | 邊境牧羊犬     | 企劃   | design | C
design-B-1 | 香蕉     | 黃金獵犬       | 美術   | design | B
design-B-2 | 小玲     | 臘腸           | 美術   | design | B
design-B-3 | ㄧ加     | 伯恩山犬       | 企劃   | design | B
design-A-1 | 小成     | 阿富汗獵犬     | 美術   | design | A
design-A-2 | 恐龍     | 邊境牧羊犬     | 企劃   | design | A
design-S-1 | 嚕比醬   | 薩摩耶         | 美術   | design | S
```

#### marketing 行銷業務（16）

```
mkt-D-1 | Max      | 巴哥           | 業務   | marketing | D
mkt-D-2 | Coco     | 吉娃娃         | 業務   | marketing | D
mkt-D-3 | 呱呱     | 比熊           | 業務   | marketing | D
mkt-D-4 | 貼貼     | 柯基           | 行銷   | marketing | D
mkt-D-5 | 宇宙     | 波士頓㹴       | 行銷   | marketing | D
mkt-D-6 | 毛毛     | 蝴蝶犬         | 行銷   | marketing | D
mkt-C-1 | Peeta    | 哈士奇         | 業務   | marketing | C
mkt-C-2 | 桃桃     | 黃金獵犬       | 業務   | marketing | C
mkt-C-3 | 小安     | 雪納瑞         | 行銷   | marketing | C
mkt-C-4 | 靜靜     | 貴賓           | 行銷   | marketing | C
mkt-B-1 | KK       | 德國牧羊犬     | 業務   | marketing | B
mkt-B-2 | 哼哼     | 杜賓           | 業務   | marketing | B
mkt-B-3 | 阿瑋     | 邊境牧羊犬     | 行銷   | marketing | B
mkt-A-1 | 蘑菇     | 羅威那         | 業務   | marketing | A
mkt-A-2 | 雪莉     | 英國牧羊犬     | 行銷   | marketing | A
mkt-S-1 | 七yo     | 阿拉斯加       | 行銷   | marketing | S
```

#### service 客服（16）

```
svc-D-1 | 熙熙     | 馬爾濟斯       | 客服   | service | D
svc-D-2 | 笑笑     | 吉娃娃         | 客服   | service | D
svc-D-3 | 糖糖     | 蝴蝶犬         | 客服   | service | D
svc-D-4 | 罐頭     | 比熊           | 客服   | service | D
svc-D-5 | 小星     | 巴哥           | 客服   | service | D
svc-D-6 | 米米     | 柴犬           | 客服   | service | D
svc-C-1 | 維力     | 黃金獵犬       | 客服   | service | C
svc-C-2 | 柔柔     | 柯基           | 客服   | service | C
svc-C-3 | Flash    | 惠比特         | 客服   | service | C
svc-C-4 | 百變     | 貴賓           | 客服   | service | C
svc-B-1 | York     | 伯恩山犬       | 客服   | service | B
svc-B-2 | 魚魚     | 英國牧羊犬     | 客服   | service | B
svc-B-3 | 典典     | 德國牧羊犬     | 客服   | service | B
svc-A-1 | 水獺     | 伯恩山犬       | 客服   | service | A
svc-A-2 | Yuna     | 邊境牧羊犬     | 客服   | service | A
svc-S-1 | 露西亞   | 大白熊         | 客服   | service | S
```

#### U 級（1）

```
u-1 | 任勞任怨狗 | 鬆獅犬 | CEO | tech | U
```

---

**完整白名單（複製給後端用，65 筆）**：

```
tech-D-1, tech-D-2, tech-D-3, tech-D-4, tech-D-5, tech-D-6,
tech-C-1, tech-C-2, tech-C-3, tech-C-4,
tech-B-1, tech-B-2, tech-B-3,
tech-A-1, tech-A-2,
tech-S-1,
design-D-1, design-D-2, design-D-3, design-D-4, design-D-5, design-D-6,
design-C-1, design-C-2, design-C-3, design-C-4,
design-B-1, design-B-2, design-B-3,
design-A-1, design-A-2,
design-S-1,
mkt-D-1, mkt-D-2, mkt-D-3, mkt-D-4, mkt-D-5, mkt-D-6,
mkt-C-1, mkt-C-2, mkt-C-3, mkt-C-4,
mkt-B-1, mkt-B-2, mkt-B-3,
mkt-A-1, mkt-A-2,
mkt-S-1,
svc-D-1, svc-D-2, svc-D-3, svc-D-4, svc-D-5, svc-D-6,
svc-C-1, svc-C-2, svc-C-3, svc-C-4,
svc-B-1, svc-B-2, svc-B-3,
svc-A-1, svc-A-2,
svc-S-1,
u-1
```

> 來源：`src/constants/dogRoster.ts` 的 `DOG_ROSTER` 陣列。每次新增 / 修改名冊，**前端責任**同步更新此清單並用 `cp` 覆蓋到後端 repo（規則同 saves-api.md 同步）。
> 後端可以把這份白名單 hardcode 在 const，或讀同 repo 的 JSON。

---

### Team 子物件結構（v3）

```json
{
  "category": "tech",
  "level": 3,
  "exp": 24,
  "memberDogIds": ["dog_42", "dog_15"]
}
```

server 不需驗欄位細節，照單儲存。陣列長度 ≤ `OFFICE_LEVELS[level].maxStaff`，跟 staff 一樣寬鬆。

---

### 已棄用欄位（v1 → v2 移除 + v3 後續清理）

下列欄位在 v2/v3 不會出現，**收到也應忽略 / 拒絕**：

- `morale`（top-level，全公司士氣）→ v2 曾改用每隻 `Dog.morale`，**v3 起連 `Dog.morale` 也已移除**（疲勞、忠誠取代士氣概念）
- `health` → 改用 `reputation`
- `decor`（top-level）→ 移到 `companyBuffs.decor`
- `productivityBoost` / `stabilityBoost` / `trainingBoost` → 改用 `companyBuffs.speedBoost / qualityBoost / teamworkBoost`
- `activeChemistry` → 化學反應改成 per-project 觸發，不再持久化

---

### SaveMeta（GET / POST 成功回傳）

```json
{
  "version": 2,
  "revision": 3,
  "updated_at": "2026-04-26T12:34:56Z",
  "data": { ... }
}
```

---

## Endpoints

### GET `/saves`

取得當前使用者的 save。

**Response 200（有存檔）**：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "version": 2,
    "revision": 3,
    "updated_at": "2026-04-26T12:34:56Z",
    "data": { ... }
  }
}
```

**Response 200（無存檔）**：
```json
{ "code": 0, "message": "ok", "data": null }
```

**Response 401**：未登入 / token 失效。

---

### POST `/saves`

覆寫 current save。

**Request body**：見上面 SavePayload。

**行為**：
1. **Version 檢查**：`version ∈ {2, 3}` 才接受。`version === 1` 或 > 3 → 400 / code 4001。前端目前送 v3
2. 比對 `revision`：
   - 若資料庫沒有該 user 的 save → 允許任何 revision，視為首次存檔，server_revision 起始設為 1
   - 若有：server_revision 必須 == client_revision → 通過，server_revision += 1
   - 否則 → 409 衝突
3. Sanity check（下段）

**Response 200**（`UpsertResponse`，**不回傳 data 省頻寬**）：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "version": 2,
    "revision": 4,
    "updated_at": "2026-04-26T12:35:10Z"
  }
}
```

**Response 400**：payload 不合格式 / version 不支援 / sanity check 失敗。

**Response 409**：revision 衝突。
```json
{
  "code": 4090,
  "message": "save conflict",
  "data": {
    "server_revision": 5,
    "server_updated_at": "2026-04-26T12:36:00Z",
    "server_data": { ... }
  }
}
```

**Response 401**：未登入。

---

### DELETE `/saves`

刪除 current save。冪等（已無存檔也回 200）。

**Response 200**：
```json
{ "code": 0, "message": "ok", "data": null }
```

---

## DB Schema 建議

### `saves`

```sql
CREATE TABLE saves (
  user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  version      INT NOT NULL,
  revision     INT NOT NULL DEFAULT 1,
  data         JSONB NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

> **重寫建議**：因 v2 結構與 v1 完全不相容，可直接 `TRUNCATE saves`；或寫 migration 把 `WHERE version = 1` 的全刪。前端 client 對 v1 存檔已是「視為無效」，刪了不會有影響。

---

## Sanity Check（POST /saves 時建議執行）

**目的**：降低客端亂送成本，不追求完全防弊。

建議檢查：

1. **必要欄位非空** + 型別正確
2. **版本支援**：`version ∈ {2, 3}` 才接受
3. **數值範圍**：
   - `reputation` ∈ [0, 100]
   - `day`, `money`, `officeLevel`, `tierBudget`, `projectsCompleted`, `projectsFailed`, `bankruptCountdown` >= 0
   - `bankruptCountdown` ≤ 5
   - `tutorialStep` ∈ [0, 7]
   - `loanRepayDaysLeft` ∈ [0, 80]
   - `companyBuffs` 子欄位皆 ≥ 0（含 v3 新增的 `categorySpeed/categoryQuality/patienceBoost/fatigueRecoveryBonus`）
   - `officeSkin`（若提供）∈ [0, 4]
4. **單調遞增**（與 server 上一次 revision 比對）：
   - `day >= prev.day`（同一場遊戲天數不會倒退）
   - `projectsCompleted >= prev.projectsCompleted`（完成數不能倒退）
5. **合理性**：
   - `staff.length <= OFFICE_LEVELS[officeLevel].maxStaff + 2`（給客端超載扣分一點彈性。v2 maxStaff 表為 `[3, 5, 7, 9, 12]`）
   - 每隻 `Dog.stats` 維度 ∈ [1, 10]（接受 4 維或 5 維，多出的 `patience` 不擋）
   - 每隻 `Dog.fatigue / loyalty` ∈ [0, 100]
   - 每隻 `Dog.experience / daysAtCompany / unhappyLeaveDays / fragments` ≥ 0
   - 每隻 `Dog.level` ∈ [1, 10]（缺值視為 1）
   - 每隻 `Dog.learnedTraits.length` ≤ 8（特性表上限）
   - 每隻 `Dog.status`（若提供）∈ {`active`, `pip`}
   - 每隻 `Dog.pipDaysLeft / pipTasks.length`（若提供）≥ 0；`pipTasks.length` ≤ 10
   - 每隻 `Dog.rosterId`（若提供）必須 ∈ Dog 名冊白名單（見「Dog 名冊白名單」段落，65 筆）。**未在白名單的 rosterId → 400 / 4002**，避免客端偽造員工。`rosterId` 缺失視為合法（傳統候選人沒此欄位）
   - `clients` 內每筆 `Project.status` 為合法列舉
   - `clients.length` ≤ 30（offered 5 + active N + 最近結算 5，留餘裕）

   > 註：早期 spec 提到的 `Dog.morale` 已在前端移除，server 收到此欄位請忽略，**不要拿來做 sanity 否則永遠不觸發**。
6. **v3 only**（v2 payload 全部跳過）：
   - `tools.length` ≤ 100
   - 每筆 `Tool.grade ∈ {S, A, B}`、`Tool.category ∈ {tech, design, marketing, service}`
   - 每筆 `Tool.speedBoost / qualityBoost ∈ [0, 5]`
   - 每筆 `Tool.traits.length` ≤ 4（且全為字串）
   - `Dog.equippedToolId` 為 `string | null | undefined`，不要求一定對得上 `tools[].instanceId`（前端會清孤兒）
   - `unlockedAchievementIds.length` ≤ 50
   - `companyName.length` ≤ 8（空字串容許，代表未命名）
7. **payload 大小**：建議 server 端設上限 **192KB**（v3 多了 tools / teams / specialTasks，比 v2 大）

不過的 request → **400**，錯誤訊息指明哪個欄位。

---

## Versioning Policy

- **目前支援 `version ∈ {2, 3}`**，前端送 `3`
- v1 永久停用
- 欄位新增且有合理預設：`version` 不變，後端解析時補預設
- 欄位移除 / 欄位語意改變：`version` + 1，後端保留至少 2 版
- v2 → v3 是**純擴充**：v3 增加 optional 欄位 + `Dog.stats.patience` + `companyBuffs` 多個 boost；server 不需轉換，照單存

---

## Error Code 建議

| code | HTTP | 意義 |
|------|------|------|
| 0 | 200 | ok |
| 4000 | 400 | payload 格式錯誤 |
| 4001 | 400 | 不支援的 version（含 v1 棄用）|
| 4002 | 400 | sanity check 失敗（message 指明欄位）|
| 4010 | 401 | 未登入 / token 失效 |
| 4090 | 409 | revision 衝突 |
| 5000 | 500 | 伺服器內部錯誤 |

---

## 前端行為契約（給後端參考）

- 前端啟動 / 使用者登入後 `GET /saves`
- 進行遊戲中**每 3 天 + 關鍵動作**觸發 `POST /saves`
- 破產 / IPO 達成時會再 `POST /saves` 一次（`bankrupt=true` 或 `ipoAchievedAt!=null`），玩家按「重新開始」→ `DELETE /saves`
- 409 conflict：前端彈 modal 讓使用者選本地 or 雲端版本，不自動覆寫
- 401：前端有 auto refresh 機制
- v1 存檔：前端 `migrate()` 直接回 `null`，使用者會被視為「無存檔」從新狀態開始

---

## Open Questions

1. `data` 欄位大小上限：v3 因 `tools / teams / specialTasks` 增加，建議 server 設 **192KB** 拒絕超大 payload（v2 是 128KB，v1 是 64KB）。
2. 是否要對 v1 存檔做特殊統計（顯示「你有 X 位玩家從 v1 升級」）？目前計畫直接砍掉。
3. v3 工具系統的 `defId` / `iconName` / `traits` 是否要 server 列舉驗證？目前不驗，前端控制；若日後出現作弊問題再補。
