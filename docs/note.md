# TODO

- document_id 在两个表之间不对应
- file_path 用相对路径
- 移除重复的 file_path

D SELECT * FROM document_chunks;
┌──────────────────┬──────────────────┬──────────────────────┬──────────────────────┬──────────────┬────────────┬────────────────────────────────────────────────────────────────────────────────────┐
│        id        │   document_id    │      file_path       │         text         │ start_offset │ end_offset │                                     embedding                                      │
│     varchar      │     varchar      │       varchar        │       varchar        │    int32     │   int32    │                                      float[]                                       │
├──────────────────┼──────────────────┼──────────────────────┼──────────────────────┼──────────────┼────────────┼────────────────────────────────────────────────────────────────────────────────────┤
│ e96bbc9eb518c5de │ e96bbc9eb518c5de │ README.md            │ [简体中文](https:/…  │            0 │          0 │ [0.026928319, 0.0059297155, 0.018727256, -0.016625963, 0.028030718, -0.011874527…  │
│ d68165b50e31c74a │ d68165b50e31c74a │ en\guide\community…  │ <script setup> imp…  │            0 │          0 │ [0.003411666, 0.009626902, -0.020528182, -0.030330988, 0.023698332, 0.015994275,…  │
│ 9a630ea0aee20ef4 │ 9a630ea0aee20ef4 │ en\guide\contact.md  │ # Contact Informat…  │            0 │          0 │ [0.013832244, 0.021975003, 0.012474769, -0.0069672745, 0.02247122, -0.00544237, …  │
│ f732226f7f7ee31e │ 78710120e36bb259 │ guide\download.md    │ --- home: hello --…  │            0 │          0 │ [0.02359891, 0.0048396518, -0.035790972, -0.0077293874, 0.01566467, -0.003801493…  │
│ b16053c0efb38008 │ 78710120e36bb259 │ guide\download.md    │ .                    │            0 │          0 │ [0.010098886, 0.0032270155, 0.0024851568, 0.0058079837, 0.0038870135, 0.00340860…  │
│ 392690a5d35cb742 │ 4d9bb34f44196eba │ en\guide\download.md │ /.                   │            0 │          0 │ [0.010593739, 0.009179449, -0.0060082437, 0.003443601, 0.0016530679, 0.005412756…  │
│ ba93a49cc7f77c81 │ 78710120e36bb259 │ guide\download.md    │ /metadata.           │            0 │          0 │ [0.0023728858, 0.00084789156, 0.01189467, -0.010612184, 0.012241373, 0.01719742,…  │
│ 7cd6bfa13d7154ca │ 78710120e36bb259 │ guide\download.md    │ js' interface pack…  │            0 │          0 │ [0.006841462, 0.0016967712, 0.012908931, -0.019715264, -0.012231428, 0.005629374…  │
│ 86e45fb62aceefd4 │ 78710120e36bb259 │ guide\download.md    │ data.                │            0 │          0 │ [0.02264591, 0.018347502, -0.032731425, -0.03437506, 0.029248489, 0.014963917, -…  │
│ 0155d14983842136 │ 78710120e36bb259 │ guide\download.md    │ : string } functio…  │            0 │          0 │ [0.00074809726, -0.00439653, 0.009326323, -0.018721681, 0.007425347, 0.001732433…  │
│ fa8f1551645e54f8 │ 78710120e36bb259 │ guide\download.md    │ map(arch => { retu…  │            0 │          0 │ [0.0026626214, -0.021605968, -0.009253211, -0.0129005825, 0.019901223, 0.0175630…  │
│ f09cd61123809e96 │ 78710120e36bb259 │ guide\download.md    │ com/easytier/easyt…  │            0 │          0 │ [0.010370387, 0.0048012044, 0.026413295, -0.02238788, 0.0030410965, 0.0011491182…  │
│ fa3078e85c7cff2e │ 78710120e36bb259 │ guide\download.md    │ zip`, }, } }) } co…  │            0 │          0 │ [0.004734188, -0.013732614, 0.0064703682, -0.010998522, 0.030092033, 0.001355436…  │
│ b357935e05f02b34 │ 78710120e36bb259 │ guide\download.md    │ com/easytier/easyt…  │            0 │          0 │ [0.019256381, 0.0018826897, 0.02798331, -1.0772877e-05, 0.012519754, -0.00965317…  │
│ 7e88b3f16d220db5 │ 78710120e36bb259 │ guide\download.md    │ exe' }, cli_pkg_tm…  │            0 │          0 │ [0.0015938142, -0.0131313605, -0.0057804724, -0.012793292, 0.027615877, 0.009756…  │
│ bd2aa5ed9c38c908 │ 4d9bb34f44196eba │ en\guide\download.md │ zip' }, comment: "…  │            0 │          0 │ [0.031367682, -0.0014486907, -0.02721114, 0.0060257795, 0.008625392, -0.01721703…  │
│ 8720f4be0792d02a │ 78710120e36bb259 │ guide\download.md    │ com/easytier/easyt…  │            0 │          0 │ [0.024834797, -0.0011814053, 0.032500383, -0.014495856, 0.0026144786, -0.0017259…  │
│ bd9a5e19584f3de2 │ 78710120e36bb259 │ guide\download.md    │ " }, { os: "window…  │            0 │          0 │ [0.010467342, -0.02170721, 0.010018771, -0.006844361, 0.027166342, 0.005327859, …  │
│ a00e26f7e423007f │ 78710120e36bb259 │ guide\download.md    │ com/easytier/easyt…  │            0 │          0 │ [0.0275386, 0.0027105692, 0.024933588, -0.004679535, 0.011700359, -0.008626045, …  │
│ ddd48be352365b07 │ 78710120e36bb259 │ guide\download.md    │ com/easytier/easyt…  │            0 │          0 │ [0.01696581, 4.20251e-05, 0.022067266, -0.018678797, -0.005160758, -0.003835778,…  │
│        ·         │        ·         │        ·             │          ·           │            · │          · │                                         ·                                          │
│        ·         │        ·         │        ·             │          ·           │            · │          · │                                         ·                                          │
│        ·         │        ·         │        ·             │          ·           │            · │          · │                                         ·                                          │
│ 0ff81d9efe0d1196 │ 0ff81d9efe0d1196 │ guide\roadmap.md     │ # 路线图 - [ ] 支…   │            0 │          0 │ [-0.011828916, 0.010199746, 0.011126691, -0.0034239776, -0.0076916576, 0.0098245…  │
│ b3c172df1cca7178 │ 6c6670ae100ff3d5 │ index.md             │ dev/reference/defa…  │            0 │          0 │ [-0.004908895, 0.008260318, 0.016834037, -0.032285195, 0.011890425, 0.008005473,…  │
│ edfb3376539362de │ 6c6670ae100ff3d5 │ index.md             │ cn/web - theme: sp…  │            0 │          0 │ [0.02770243, 0.0136980545, -0.014454345, -0.017346023, 0.03282591, -0.013696164,…  │
│ 5d77b85de3a4d6a2 │ 6c6670ae100ff3d5 │ index.md             │ png' alt: 'easytie…  │            0 │          0 │ [0.018291406, -0.009850344, 0.0104685845, -0.01611523, 0.03244758, -0.012998834,…  │
│ e4767c084ac99bb5 │ 6c6670ae100ff3d5 │ index.md             │ <br>不区分客户客户…  │            0 │          0 │ [0.0036207833, 0.011876584, -0.01310598, -0.03196691, 0.018800255, -0.005336486,…  │
│ 43ffa5982e597eb7 │ 6c6670ae100ff3d5 │ index.md             │ link: /guide/netwo…  │            0 │          0 │ [0.0332554, -0.00013880403, -0.029417522, -0.019926675, 0.00042402788, 0.0063795…  │
│ 8759bd16b61b6449 │ 6c6670ae100ff3d5 │ index.md             │ icu)：由社区志愿志…  │            0 │          0 │ [0.044665933, 0.018899886, 0.01314703, 0.0035642956, 0.031165006, -7.262807e-05,…  │
│ 13717be3a2d1376a │ 6c6670ae100ff3d5 │ index.md             │ link: /guide/netwo…  │            0 │          0 │ [0.03585143, 0.012596451, -0.017409725, 0.010141684, -0.0074758986, -0.020121392…  │
│ ed99ed1d4bf30aa1 │ 6c6670ae100ff3d5 │ index.md             │ - [公共公共服务服…   │            0 │          0 │ [0.019909931, -0.013696942, 0.01774084, -0.026775917, 0.028684232, -0.008918539,…  │
│ 03781fb58618c913 │ 6c6670ae100ff3d5 │ index.md             │ - qq 群：[94970026…  │            0 │          0 │ [0.03200517, 0.0020022746, 0.0037781862, -0.018296275, 0.018970579, -0.015021156…  │
│ 80dbd46d5abf5638 │ 6c6670ae100ff3d5 │ index.md             │ cn/status/easytier…  │            0 │          0 │ [0.020629065, 0.021845305, -0.019011114, 0.0011852401, 0.007647083, -0.001564264…  │
│ e8b27e71f67e346b │ 6c6670ae100ff3d5 │ index.md             │ com/q/wfotuchqzw) …  │            0 │          0 │ [0.0349748, -0.0048544784, 0.0036462666, -0.012584868, 0.018914748, -0.009623122…  │
│ 7edb7a8f0f867f9c │ 6c6670ae100ff3d5 │ index.md             │ me/easytier ## 鸣…   │            0 │          0 │ [0.018768229, -0.016086245, 0.025987374, -0.017657764, 0.031428922, -9.317656e-0…  │
│ c3c063c595d46688 │ 6c6670ae100ff3d5 │ index.md             │ 95rem; color: blac…  │            0 │          0 │ [0.018094623, -0.007827087, -0.0052801315, -0.019840935, 0.028376032, 0.00057597…  │
│ 3e593891a48b5c3f │ 6c6670ae100ff3d5 │ index.md             │ png" alt="浪浪云" …  │            0 │          0 │ [0.012421325, 0.020278431, 0.012257947, 0.000999684, 0.012332039, 0.011362947, 0…  │
│ c12f3f1207e90827 │ 6c6670ae100ff3d5 │ index.md             │ png" alt="雨云" st…  │            0 │          0 │ [0.02030894, 0.026985953, 0.011339716, -0.001777246, 0.008768378, 0.006284152, 0…  │
│ f335dcc52a4c83eb │ 6c6670ae100ff3d5 │ index.md             │ 95rem; color: blac…  │            0 │          0 │ [0.007839765, -0.0049026296, 0.01390792, -0.03634995, 0.018450055, 0.020388136, …  │
│ 8f9949128c3ade78 │ 6c6670ae100ff3d5 │ index.md             │ 软件的开发和维护需…  │            0 │          0 │ [0.04049144, -7.536426e-05, 0.01612817, -0.0070344037, 0.037985656, 0.0100545585…  │
│ e302e728aaa758c1 │ 6c6670ae100ff3d5 │ index.md             │ png" alt="微信" st…  │            0 │          0 │ [0.004173658, -0.0065787253, 0.020238074, -0.02330515, 0.013416675, 0.014135229,…  │
│ 4bd6c9d1296088cf │ 6c6670ae100ff3d5 │ index.md             │ png" alt="支付支付…  │            0 │          0 │ [0.019452142, 0.008578573, -0.01517305, -0.016965557, 0.027077083, 0.0081686685,…  │
├──────────────────┴──────────────────┴──────────────────────┴──────────────────────┴──────────────┴────────────┴────────────────────────────────────────────────────────────────────────────────────┤
│ 1392 rows (40 shown)                                                                                                                                                                     7 columns │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
D SELECT * FROM processed_files;
┌──────────────────────────────────────────────────────────────────────────────────────────────┬──────────────────┬─────────────────────────┐
│                                          file_path                                           │    file_hash     │      processed_at       │
│                                           varchar                                            │     varchar      │        timestamp        │
├──────────────────────────────────────────────────────────────────────────────────────────────┼──────────────────┼─────────────────────────┤
│ E:\workspace\EasyTier\easytier.github.io\README.md                                           │ f5d136262a036946 │ 2025-08-10 00:07:16.838 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\community-and-contribution.md              │ 0f7fd50770c1c98e │ 2025-08-10 00:07:16.849 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\contact.md                                 │ c6ea53c6baddec09 │ 2025-08-10 00:07:16.86  │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\download.md                                │ afd097c156479d2b │ 2025-08-10 00:07:16.971 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\faq.md                                     │ 9c86760f52adb56f │ 2025-08-10 00:07:17.011 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\astral-game.md                         │ e4aea4f962fbb1d2 │ 2025-08-10 00:07:17.052 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\basic.md                               │ d8db7fc8de6ce48b │ 2025-08-10 00:07:17.077 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\easytier-game.md                       │ 6d4e789452f1cc6b │ 2025-08-10 00:07:17.137 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\easytier-manager.md                    │ 2041b4b7babfc617 │ 2025-08-10 00:07:17.202 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\index.md                               │ 6211d26b1a5aeffd │ 2025-08-10 00:07:17.211 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\manual.md                              │ 1a0738c8c0937d3b │ 2025-08-10 00:07:17.22  │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\subnet_proxy.md                        │ f99ddda730b6951b │ 2025-08-10 00:07:17.252 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\gui\vpn_portal.md                          │ 0cc636024c46bb9c │ 2025-08-10 00:07:17.273 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\installation.md                            │ f24b475fa6c504c7 │ 2025-08-10 00:07:17.339 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\installation_gui.md                        │ e9271ea5c3842e23 │ 2025-08-10 00:07:17.381 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\introduction.md                            │ 630c1bf6c74c70cb │ 2025-08-10 00:07:17.421 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\license.md                                 │ a667a25f760792a2 │ 2025-08-10 00:07:17.431 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\network\config-file.md                     │ 4b30db23b1aec246 │ 2025-08-10 00:07:17.458 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\network\configurations.md                  │ 710b37e3310c7151 │ 2025-08-10 00:07:17.606 │
│ E:\workspace\EasyTier\easytier.github.io\en\guide\network\decentralized-networking.md        │ 745aaa6191256953 │ 2025-08-10 00:07:17.676 │
│                                      ·                                                       │        ·         │            ·            │
│                                      ·                                                       │        ·         │            ·            │
│                                      ·                                                       │        ·         │            ·            │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\host-public-server.md                 │ 07c0cdd1c94eb495 │ 2025-08-10 00:07:19.424 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\install-as-a-macos-service.md         │ dbc650be14a88cc7 │ 2025-08-10 00:07:19.441 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\install-as-a-systemd-service.md       │ 30df47158ae99601 │ 2025-08-10 00:07:19.468 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\install-as-a-windows-service.md       │ b856cb0be7c8bda9 │ 2025-08-10 00:07:19.525 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\kcp-proxy.md                          │ 0a6b18e1d3677ee5 │ 2025-08-10 00:07:19.586 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\magic-dns.md                          │ 5e0e95147d0184bb │ 2025-08-10 00:07:19.594 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\network-to-network.md                 │ df4811877954e7b1 │ 2025-08-10 00:07:19.678 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\networking-without-public-ip.md       │ d4e3ee78190e1c93 │ 2025-08-10 00:07:19.721 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\no-root.md                            │ e73622b554fe6aa4 │ 2025-08-10 00:07:19.728 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\oneclick-install-as-service.md        │ e7f98b6601b876f4 │ 2025-08-10 00:07:19.759 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\p2p-optimize.md                       │ 73b706b23957d37e │ 2025-08-10 00:07:19.801 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\point-to-networking.md                │ 8667d4b17254b5bf │ 2025-08-10 00:07:19.951 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\quick-networking.md                   │ 998ef8ec00a78b3e │ 2025-08-10 00:07:20.067 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\socks5.md                             │ d59ac694fdc5782b │ 2025-08-10 00:07:20.074 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\use-easytier-with-wireguard-client.md │ 0f71365507029913 │ 2025-08-10 00:07:20.155 │
│ E:\workspace\EasyTier\easytier.github.io\guide\network\web-console.md                        │ 0f258a006e30cee6 │ 2025-08-10 00:07:20.191 │
│ E:\workspace\EasyTier\easytier.github.io\guide\networking.md                                 │ 48aee134beb8cea7 │ 2025-08-10 00:07:20.199 │
│ E:\workspace\EasyTier\easytier.github.io\guide\perf.md                                       │ 8c744a9e767b0da3 │ 2025-08-10 00:07:20.384 │
│ E:\workspace\EasyTier\easytier.github.io\guide\roadmap.md                                    │ 53f36dcaae79a407 │ 2025-08-10 00:07:20.391 │
│ E:\workspace\EasyTier\easytier.github.io\index.md                                            │ 7ea253b6f44bc575 │ 2025-08-10 00:07:20.445 │
├──────────────────────────────────────────────────────────────────────────────────────────────┴──────────────────┴─────────────────────────┤
│ 79 rows (40 shown)                                                                                                              3 columns │
└───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
D SELECT * FROM processed_files;

fanya@fanmi-nuc-rog E:\workspace\SlimRAG\SlimRAG git:(main ~1)
(0) > duckdb .\srag.ddb
DuckDB v1.3.2 (Ossivalis) 0b83e5d2f6
Enter ".help" for usage hints.
D SHOW TABLES;
┌─────────────────┐
│      name       │
│     varchar     │
├─────────────────┤
│ document_chunks │
│ processed_files │
│ rag_metadata    │
└─────────────────┘
D SELECT * FROM rag_metadata;
┌─────────┬─────────┐
│   key   │  value  │
│ varchar │ varchar │
├─────────┴─────────┤
│      0 rows       │
└───────────────────┘
