# Game Server Join Protocol Coverage

Fixture: the Test cell ends in `_Joined` while Depth is `QUERY`, which check 16
must reject. Status is `blocked-doc` on purpose — a `covered-*` status would trip
check 5 (status/depth consistency) first and mask the check under test.

| Module | Game | Status | Depth | Test | Bucket | Last Verified | Blocker | Blocker Class |
|---|---|---|---|---|---|---|---|---|
| `minecraft-java` | Minecraft: Java Edition | blocked-doc | QUERY | TestGameServer_MinecraftJavaBot_Joined | bot-fast | — | Wire format undocumented; needs a packet capture of a real client join | documentation |
