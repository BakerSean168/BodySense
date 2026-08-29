# Changelog

## [0.9.0](https://github.com/BakerSean168/BodySense/compare/v0.8.1...v0.9.0) (2026-08-29)


### Features

* **ai:** enforce durable consultation state acquisition ([2ea7c16](https://github.com/BakerSean168/BodySense/commit/2ea7c16d9b1a0a4efbafe05dc4cf58e144d01817))
* enforce durable consultation state acquisition ([fb8ca26](https://github.com/BakerSean168/BodySense/commit/fb8ca2656145fa955704da403f7cfac949acb8c5))


### Bug Fixes

* **ai:** drop optional consultation question tails ([df8dd5a](https://github.com/BakerSean168/BodySense/commit/df8dd5ac37c50232185efd1832cca5ca9b594e90))
* **ai:** enforce interaction-only followup questions ([e6856a8](https://github.com/BakerSean168/BodySense/commit/e6856a8ce4ea74629e726049d650cd9c923b909a))
* **ai:** ignore explanatory question examples ([0f0ba43](https://github.com/BakerSean168/BodySense/commit/0f0ba436ce60e6714d66debe9f9f2bf0dc2b208f))
* **ai:** ignore rhetorical consultation headings ([577af47](https://github.com/BakerSean168/BodySense/commit/577af47481d3dc5b012e46b997da6b44206b6599))
* **ai:** isolate consultation safety input ([b8e2c0c](https://github.com/BakerSean168/BodySense/commit/b8e2c0c753814c8d653c90e39fb167081f3d3bb0))
* **ai:** route consultation questions through interactions ([48ab616](https://github.com/BakerSean168/BodySense/commit/48ab6164a2a2bdba4f54711b74778e4041fd9ef9))

## [0.8.1](https://github.com/BakerSean168/BodySense/compare/v0.8.0...v0.8.1) (2026-08-28)


### Bug Fixes

* **ops:** bootstrap schema before ai on fresh pg18 ([bbe9073](https://github.com/BakerSean168/BodySense/commit/bbe9073463b458e74cf231dccc3d9b428fc44dd0))
* **ops:** bootstrap schema before ai on fresh pg18 ([a105818](https://github.com/BakerSean168/BodySense/commit/a1058189a628717a594b34550b6aa18666fd7c3c))

## [0.8.0](https://github.com/BakerSean168/BodySense/compare/v0.7.2...v0.8.0) (2026-08-28)


### Features

* **ops:** standardize production on postgres 18 ([782952d](https://github.com/BakerSean168/BodySense/commit/782952df145a799fca69600ea58f501aa2031f3b))
* **ops:** standardize production on postgres 18 ([cad8a00](https://github.com/BakerSean168/BodySense/commit/cad8a00f069185ab4c0a2a81b1e2d726dc87d579))

## [0.7.2](https://github.com/BakerSean168/BodySense/compare/v0.7.1...v0.7.2) (2026-08-27)


### Bug Fixes

* **ops:** harden capacity status collection ([362e02e](https://github.com/BakerSean168/BodySense/commit/362e02e48b3c99c61e14d5a9005cd41f1f44957f))
* **ops:** harden capacity status collection ([e516b5b](https://github.com/BakerSean168/BodySense/commit/e516b5bf81c4656f232a7e8f68f9aa4945350365))

## [0.7.1](https://github.com/BakerSean168/BodySense/compare/v0.7.0...v0.7.1) (2026-08-27)


### Bug Fixes

* **ops:** tolerate swap metadata page overhead ([f88413f](https://github.com/BakerSean168/BodySense/commit/f88413fc1774fa0a02cd73a7cc40cc4a7b02c1d6))
* **ops:** tolerate swap metadata page overhead ([0464abf](https://github.com/BakerSean168/BodySense/commit/0464abf0463d16fee04d431aed0e54732e912a6f))

## [0.7.0](https://github.com/BakerSean168/BodySense/compare/v0.6.0...v0.7.0) (2026-08-27)


### Features

* **main:** integrate longitudinal health and 3d explorer ([4b19db6](https://github.com/BakerSean168/BodySense/commit/4b19db67d16addfedb5bf54a37f3309185743406))
* **main:** integrate longitudinal health and 3d explorer ([9524e77](https://github.com/BakerSean168/BodySense/commit/9524e7759d64db0b4e0c5219d9f9540abbb507c6))


### Bug Fixes

* **docker:** make private-destination preflight fail closed on malformed IsPublic (BS-PROD-012) ([211ea1d](https://github.com/BakerSean168/BodySense/commit/211ea1d05cc00c2754014c498303a37b603bd594))
* **docker:** require the drill network to contain exactly the restore container (BS-PROD-012) ([93cc5ae](https://github.com/BakerSean168/BodySense/commit/93cc5ae82ccc4c930756cacfd813e01343e064b5))
* **docker:** restore reviewed off-host DR into combined runtime bundle (BS-PROD-012) ([910b5ce](https://github.com/BakerSean168/BodySense/commit/910b5ce6bd26bfb8f7a43e6515f09cfdc16291e1))
* **docker:** run restore validators in a disposable container (BS-PROD-012) ([734019a](https://github.com/BakerSean168/BodySense/commit/734019a97f8a08de0eeb4470de403830880b26d1))
* **ops:** capture off-host DR freshness output before asserting OK marker (BS-PROD-012) ([f3c641f](https://github.com/BakerSean168/BodySense/commit/f3c641ffaa4eb0de7fc2997e6abfffaffc9ac46c))
* **ops:** make private-destination policy-status preflight fail closed on malformed IsPublic (BS-PROD-012) ([9672d33](https://github.com/BakerSean168/BodySense/commit/9672d335f7be5e8c6ce376def479532daa14ab67))
* **ops:** reconcile target branch into DR delivery without conflict ([539aca3](https://github.com/BakerSean168/BodySense/commit/539aca3f625e893a39915bab8113dce9775b10ed))
* **ops:** reconcile target branch into DR delivery without conflict ([6dff1c1](https://github.com/BakerSean168/BodySense/commit/6dff1c169d254123fe37f3a51d134be750651af3))
* **ops:** require the drill network to contain exactly the restore container, fail on any other member (BS-PROD-012) ([f0c7d34](https://github.com/BakerSean168/BodySense/commit/f0c7d342dc4c5089e1364fdaa15970e78e9ec74b))
* **ops:** restore reviewed off-host DR into combined runtime bundle (BS-PROD-012) ([b4d811e](https://github.com/BakerSean168/BodySense/commit/b4d811ecef1113ab3b9478b5320d48ca0556eb2b))
* **ops:** run restore validators in a disposable container, refuse production members on the drill network (BS-PROD-012) ([9fb3f5b](https://github.com/BakerSean168/BodySense/commit/9fb3f5ba02bf76c1261158c5da02b495bd9dec04))
* **web:** allow configured Tailnet Vite hosts ([af310c9](https://github.com/BakerSean168/BodySense/commit/af310c9dc30d22628f1aa9883388fdd122343429))
* **web:** allow configured Tailnet Vite hosts ([79faf3f](https://github.com/BakerSean168/BodySense/commit/79faf3f3d64412a626f6f2189bdb7cbac445d1a7))

## [0.6.0](https://github.com/BakerSean168/BodySense/compare/v0.5.2...v0.6.0) (2026-08-25)


### Features

* add durable user upload storage ([5443db2](https://github.com/BakerSean168/BodySense/commit/5443db29de363502156866318b939bd53d6af0d9))
* add durable user upload storage ([b703de4](https://github.com/BakerSean168/BodySense/commit/b703de4a84602c8fefe5ecbf17519cafa6f26bb9))
* add off-host postgres disaster recovery ([941261d](https://github.com/BakerSean168/BodySense/commit/941261d4e331b8416c4bf2f04f1664c03600cec5))
* add off-host postgres disaster recovery ([de8f712](https://github.com/BakerSean168/BodySense/commit/de8f712654d08bee56ef8267e69d56fc3914cda6))
* **ai:** govern external evidence admissibility ([aacbd08](https://github.com/BakerSean168/BodySense/commit/aacbd08ef505901f45421c6ef3ddc1984710c77f))
* **ai:** govern external evidence admissibility ([4295e10](https://github.com/BakerSean168/BodySense/commit/4295e10f3f143b7fca4450a1f52f042e4217e4ed))
* **ai:** ingest Thought Forest health snapshots ([45804a0](https://github.com/BakerSean168/BodySense/commit/45804a004a0a930c05e93a4a967acc5521c7f69b))
* **ai:** ingest Thought Forest health snapshots ([791f5fd](https://github.com/BakerSean168/BodySense/commit/791f5fdc2e52b2d870a93deb7ef630ef2fa2da6d))
* **ai:** normalize Thought Forest evidence identity ([a56317f](https://github.com/BakerSean168/BodySense/commit/a56317f4ba6e6b57220f8a68d617934f8a5e0d14))
* **ai:** normalize Thought Forest evidence identity ([6da11f5](https://github.com/BakerSean168/BodySense/commit/6da11f5dad9102d30ee8b99c623bbb3ffb6b1cb9))
* **api:** add durable knowledge ingestion jobs ([3c240bf](https://github.com/BakerSean168/BodySense/commit/3c240bf352e1df20ee60d59d3c954c47afbdc304))
* **api:** govern knowledge operator registration ([3d86c8a](https://github.com/BakerSean168/BodySense/commit/3d86c8aab4a8636ec4092d379647be8b4cedec31))
* **api:** harden knowledge publication governance ([6ac68ff](https://github.com/BakerSean168/BodySense/commit/6ac68ff74274e5f4403356cce263c22f13e50794))
* **api:** make evidence availability explicit ([7235db8](https://github.com/BakerSean168/BodySense/commit/7235db8a7fc03b0e760cafaf00f5fe00048c8c89))
* close out knowledge governance and split durability plan ([0bd5170](https://github.com/BakerSean168/BodySense/commit/0bd517022968e0a478e43a08545991324530ba57))
* govern claim publication and rollback ([62d3e73](https://github.com/BakerSean168/BodySense/commit/62d3e73ea4753beab8ff53d050a36bbd3343b68a))
* govern claim publication and rollback ([0630f4f](https://github.com/BakerSean168/BodySense/commit/0630f4fd469a629d76dc3f81bfbe9adfe04c7a2a))
* govern published knowledge quality ([07cefd1](https://github.com/BakerSean168/BodySense/commit/07cefd107a6c35a8c9a29c1fad0e6315e1db006d))
* govern published knowledge quality ([3d13f58](https://github.com/BakerSean168/BodySense/commit/3d13f587551de115289cfee5dfe2bcc20aa3ddc6))
* harden production capacity baseline ([7c915fd](https://github.com/BakerSean168/BodySense/commit/7c915fd5cb7de1f2aa1f872239f72aaf58f632a9))
* harden production capacity baseline ([696d00d](https://github.com/BakerSean168/BodySense/commit/696d00d4c4a732426a66a8740f513be07f1ed8df))
* observe published knowledge at runtime ([92be7dd](https://github.com/BakerSean168/BodySense/commit/92be7dd590c02a2c76aa9c3f51fe69a9de0a94b2))
* observe published knowledge at runtime ([ba4d7ff](https://github.com/BakerSean168/BodySense/commit/ba4d7ffc6bb78ed985140587539e523db34a869d))
* redesign project showcase ([802e27c](https://github.com/BakerSean168/BodySense/commit/802e27c77b3608927b28c4d757466fad396a73fb))
* redesign project showcase ([0ac8174](https://github.com/BakerSean168/BodySense/commit/0ac817418ab11fa8b9c69b1bb119e3829285c2f2))
* rollout reviewed knowledge cohorts ([e8b3775](https://github.com/BakerSean168/BodySense/commit/e8b37750bc0a45930fa6b68ff06a79fd0ac438e9))
* rollout reviewed knowledge cohorts ([3986501](https://github.com/BakerSean168/BodySense/commit/398650177fdf0d5b72cfd02b6e1ac45b29457f95))
* **web:** implement durable run recovery, cancel, and deploy preflight ([80de45c](https://github.com/BakerSean168/BodySense/commit/80de45cbb734de2ab4ffd248527e07e0606435c0))


### Bug Fixes

* **ai:** satisfy external evidence typecheck ([c05569c](https://github.com/BakerSean168/BodySense/commit/c05569c816c0f9da20e17605f7616143cd893f18))
* **api:** atomically revoke shares when deleting a conversation ([5770a44](https://github.com/BakerSean168/BodySense/commit/5770a44b2f7e5f38b5f6a43bbb283502161f073a))
* **api:** complete consultation run lease lifecycle ([5b0f8e0](https://github.com/BakerSean168/BodySense/commit/5b0f8e03d42cb280e9486b3eeb9c0ce3e77f87c4))
* **api:** enforce conversation ownership for interaction metrics ([efdc72f](https://github.com/BakerSean168/BodySense/commit/efdc72feb09b04221e67c8197a740f688760c4e4))
* **api:** expose run lease repository capabilities ([f825fd9](https://github.com/BakerSean168/BodySense/commit/f825fd9ad1d41426c7d91b02f2ce74cab2ef1b05))
* **api:** lease runs and reclaim stale executions on process death ([a12735e](https://github.com/BakerSean168/BodySense/commit/a12735eb313d52ccc191e2e8d58cdbc2d5574781))
* **api:** make refresh rotation replay-safe ([81f5be3](https://github.com/BakerSean168/BodySense/commit/81f5be308739d628fdc9b4ec968c5fe2a7bd02d9))
* **api:** short-lived access tokens and session-family revocation ([cbf1606](https://github.com/BakerSean168/BodySense/commit/cbf1606d1c77b35bdce0e1db54ddf5557f7c8bf7))
* harden production security and runtime recovery ([3fc911e](https://github.com/BakerSean168/BodySense/commit/3fc911e1d606bbbd9b19a52b8960abbc775f40cb))
* **main:** add durable privacy erasure boundary ([423c910](https://github.com/BakerSean168/BodySense/commit/423c91015522e7a84f46a099e2ebe5966f059314))
* **main:** harden browser session boundary ([ccbe067](https://github.com/BakerSean168/BodySense/commit/ccbe067fe30e7f5758f4d6d0f61bf8e0aad5fc11))
* preserve cancelled consultation state ([04377c9](https://github.com/BakerSean168/BodySense/commit/04377c9c0548e44997658ec25203a24b146eb64f))
* recover consultation runs after api restart ([1d0586e](https://github.com/BakerSean168/BodySense/commit/1d0586ec0d76343a8a7a36536e35cc290ffa084f))

## [0.5.2](https://github.com/BakerSean168/BodySense/compare/v0.5.1...v0.5.2) (2026-08-23)


### Bug Fixes

* **docker:** publish multiarch production images ([71eee75](https://github.com/BakerSean168/BodySense/commit/71eee758f37e9cb987c2dabdbbac892caf6250b4))
* **docker:** publish multiarch production images ([07d7169](https://github.com/BakerSean168/BodySense/commit/07d7169b3c64d98e57f0812c066df9d85bd2b3f6))
* **docker:** restore aliyun production topology ([361ca35](https://github.com/BakerSean168/BodySense/commit/361ca35ea32b2d6eb1f1dd36ee33ef485e7287aa))
* **docker:** restore aliyun production topology ([416b2f9](https://github.com/BakerSean168/BodySense/commit/416b2f9d83902a715ce520971967a1824166430a))

## [0.5.1](https://github.com/BakerSean168/BodySense/compare/v0.5.0...v0.5.1) (2026-08-23)


### Bug Fixes

* **docker:** support shared edge production hosts ([d6fbff4](https://github.com/BakerSean168/BodySense/commit/d6fbff4b61f8f52ef9ac3d42910f8ac13f9b909f))
* **docker:** support shared edge production hosts ([1a84679](https://github.com/BakerSean168/BodySense/commit/1a84679e659d36f38cb7a777a1a58d8450a3c391))

## [0.5.0](https://github.com/BakerSean168/BodySense/compare/v0.4.3...v0.5.0) (2026-08-22)


### Features

* **main:** harden consultation runtime and RAG boundaries ([#91](https://github.com/BakerSean168/BodySense/issues/91)) ([142d8de](https://github.com/BakerSean168/BodySense/commit/142d8de00f179a00aa837bcc820e66daa8cf3e59))

## [0.4.3](https://github.com/T1moooo/BodySense/compare/v0.4.2...v0.4.3) (2026-08-22)


### Bug Fixes

* **docker:** package production runtime in ACR ([5e2b1e1](https://github.com/T1moooo/BodySense/commit/5e2b1e1f23532b98cc5d0cd3c6ae71823d37d6a4))
* **docker:** package production runtime in ACR ([b01cecb](https://github.com/T1moooo/BodySense/commit/b01cecb7a7b0961311b730ad6a776ac6c174d4a6))

## [0.4.2](https://github.com/T1moooo/BodySense/compare/v0.4.1...v0.4.2) (2026-08-22)


### Bug Fixes

* create production runtime directories before sync ([#75](https://github.com/T1moooo/BodySense/issues/75)) ([d0ca2e8](https://github.com/T1moooo/BodySense/commit/d0ca2e83ea42d38d1eadd6990a2d42933772b853))

## [0.4.1](https://github.com/T1moooo/BodySense/compare/v0.4.0...v0.4.1) (2026-08-22)


### Bug Fixes

* **api:** restore published migration 29 compatibility ([c9093fc](https://github.com/T1moooo/BodySense/commit/c9093fce832b98c3129c4a29e6378cf6f0b0e566))
* **ci:** harden Alibaba production delivery ([d7c986d](https://github.com/T1moooo/BodySense/commit/d7c986d530465004a6c00e4779888c00015fc0d5))

## [0.4.0](https://github.com/T1moooo/BodySense/compare/v0.3.0...v0.4.0) (2026-08-22)


### Features

* **ai:** add consultation eval harness ([ec42c2d](https://github.com/T1moooo/BodySense/commit/ec42c2dc8eb00fd1483be0ef68649189495d1b31))
* **ai:** add diagnosis qualification eval plane ([a4d12c9](https://github.com/T1moooo/BodySense/commit/a4d12c944f499bcfdff617f953cf526cda4b8321))
* **ai:** add diagnosis qualification eval plane ([f8bc1c1](https://github.com/T1moooo/BodySense/commit/f8bc1c154cf7743199f20cb0d8c47047f4b8850e))
* **ai:** add immutable Assessment agent configuration ([df37850](https://github.com/T1moooo/BodySense/commit/df378503910bc7718bf1b98150989a76acee83b1))
* **ai:** add immutable consultation agent configuration (Phase 1) ([5c2bc1d](https://github.com/T1moooo/BodySense/commit/5c2bc1d22355c7b6126f5f2701e3bc65ebbd854f))
* **ai:** add posture analysis route, analyzer, prompt, and governance ([0a46317](https://github.com/T1moooo/BodySense/commit/0a46317a4c3fcea8446652b5de6d473163052921))
* **ai:** add standalone litellm gateway ([322aa35](https://github.com/T1moooo/BodySense/commit/322aa35d5af351470eedb9219490e07ee7378c54))
* **ai:** add standalone LiteLLM gateway ([9776a80](https://github.com/T1moooo/BodySense/commit/9776a80becf2fda7d40f8220b264a3528a5d6fdc))
* **ai:** add treatment agent configuration ([253081b](https://github.com/T1moooo/BodySense/commit/253081bda0411c14d87ef26ae5c6291fe58d61b5))
* **ai:** add treatment agent configuration ([4aded57](https://github.com/T1moooo/BodySense/commit/4aded57d5cfe07ee27f1512f79f6b86ef22b93b8))
* **ai:** add treatment evidence gap challenger ([2c36a90](https://github.com/T1moooo/BodySense/commit/2c36a9035a73d58f8b6731533bf49ca97afad351))
* **ai:** add treatment evidence gap challenger ([af7a9f1](https://github.com/T1moooo/BodySense/commit/af7a9f19a21212c95d811936e75c9337900acb2a))
* **ai:** Assessment agent platform — immutable config + runtime provenance (Phase 1-2) ([1a7411b](https://github.com/T1moooo/BodySense/commit/1a7411b55ad8408ef2ec1f26c438b7a591b9232f))
* **ai:** consultation manifest-driven model boundary (Phase 6) ([b24c829](https://github.com/T1moooo/BodySense/commit/b24c82941bc81e7fc682a19bb4fa8b7e1ebd191b))
* **ai:** consultation runtime resolves immutable manifest + emits provenance (Phase 2) ([702f013](https://github.com/T1moooo/BodySense/commit/702f01343164875a46ba28b79afba19fecb29252))
* **ai:** enforce output governance on diagnosis and treatment ([e3e9c1b](https://github.com/T1moooo/BodySense/commit/e3e9c1bf751641713805e73104d7a3e14e5e5961))
* **ai:** integrate posture analysis into consultation runtime ([ed2d1fb](https://github.com/T1moooo/BodySense/commit/ed2d1fb5d5b37ef2be6fe78ce7c8d8ff3cb00413))
* **ai:** resolve Assessment through immutable agent configuration ([479c125](https://github.com/T1moooo/BodySense/commit/479c12538d1798a7a12302b32f04d327964b7ce5))
* **api:** add assessment frozen-input replay and comparison (Phase 4) ([1110a0f](https://github.com/T1moooo/BodySense/commit/1110a0f09482d2b3dbf55007cfd534be9d759a1a))
* **api:** add assessment go ownership - deployment policy, provenance, decision trace ([a0572bf](https://github.com/T1moooo/BodySense/commit/a0572bf25745e887e9ec192e590db2580a0f6804))
* **api:** add assessment rollout governance (Phase 5) ([45520de](https://github.com/T1moooo/BodySense/commit/45520de50ed972f5925fe72d51a434a45931bb61))
* **api:** add deterministic diagnosis decision authority ([465dbcf](https://github.com/T1moooo/BodySense/commit/465dbcfc20aef2d081480faa2f1ef2a2100a9d49))
* **api:** add deterministic diagnosis decision authority ([c43e123](https://github.com/T1moooo/BodySense/commit/c43e123cf9573e68f65a1d04cd3fb82d2cfadfdb))
* **api:** add diagnosis promotion governance ([64d916e](https://github.com/T1moooo/BodySense/commit/64d916eac60bc95f80496135f5caff68d7dda097))
* **api:** add diagnosis promotion governance ([700583a](https://github.com/T1moooo/BodySense/commit/700583af862f9da0a52a3a827f2dff9c64ad5269))
* **api:** add diagnosis replay comparison ([ff4f412](https://github.com/T1moooo/BodySense/commit/ff4f41250302dfd54532b65719af504d3ac4126d))
* **api:** add diagnosis replay comparison ([af23abb](https://github.com/T1moooo/BodySense/commit/af23abb8000efc3f16b3f730a2490dd0503247af))
* **api:** add posture analysis columns and repository methods to user_uploads ([8b78f10](https://github.com/T1moooo/BodySense/commit/8b78f10b055a7514e7b9f5e21adfbbb52e9cb7a7))
* **api:** add treatment decision traces ([f11ede6](https://github.com/T1moooo/BodySense/commit/f11ede641a484651553a81f10c0a094c7f2512d7))
* **api:** add treatment decision traces ([5f6645d](https://github.com/T1moooo/BodySense/commit/5f6645d8fcf0a208efc3c78a8c50feb405d13a47))
* **api:** add treatment replay comparison ([8148d03](https://github.com/T1moooo/BodySense/commit/8148d036cf6cec5354b0fcb2259e77c6c5161265))
* **api:** add treatment replay comparison ([a7bbad5](https://github.com/T1moooo/BodySense/commit/a7bbad54c0d593e698cbdba7264cba9a4a1826ac))
* **api:** add treatment rollout governance ([6716227](https://github.com/T1moooo/BodySense/commit/671622765600e5719cb74dfd0467806abcf42c9e))
* **api:** add treatment rollout governance ([51e3983](https://github.com/T1moooo/BodySense/commit/51e398392cf941f28fcd2f06402418114af84511))
* **api:** Assessment frozen-input replay and comparison (Phase 4) ([fd77b2b](https://github.com/T1moooo/BodySense/commit/fd77b2bea2446fc27d809d636c6354e7f50a65ae))
* **api:** Assessment Go ownership - deployment policy, provenance, decision trace (Phase 3) ([6119f59](https://github.com/T1moooo/BodySense/commit/6119f59e5a649b577c5d8428ce22d2076ce46171))
* **api:** consultation Go ownership - deployment policy + config wiring (Phase 3) ([db0a6fc](https://github.com/T1moooo/BodySense/commit/db0a6fcd181008be052c66a57506d743cbcea6d1))
* **api:** consultation rollout governance (Phase 5) ([aadfe6c](https://github.com/T1moooo/BodySense/commit/aadfe6c00542b04f92e37575f9074ec97787f154))
* **api:** consultation run provenance persistence + deployment wiring (Phase 3-4) ([2399c90](https://github.com/T1moooo/BodySense/commit/2399c909f459c5c412a83820baac8008957313c2))
* **api:** consultation run-level decision replay (Phase 4) ([592c9fd](https://github.com/T1moooo/BodySense/commit/592c9fd43f40941a9be55a61c055ff2a98a2aa89))
* **api:** enqueue and process posture-analyze jobs for photo uploads ([bc9a638](https://github.com/T1moooo/BodySense/commit/bc9a63846f6ac616d4364ee2daba21463dfd9169))
* **api:** harden consultation runtime ledger and HITL lifecycle ([cff6945](https://github.com/T1moooo/BodySense/commit/cff69457f53a860fc2600ca5eddf50dae5e36388))
* **api:** persist diagnosis decision provenance ([26a6b35](https://github.com/T1moooo/BodySense/commit/26a6b3520b56e3ad597e9313a10982f50abe3d2e))
* **api:** persist diagnosis decision provenance ([6027936](https://github.com/T1moooo/BodySense/commit/6027936286bfd64ef43e77cc6c43615dd1f72362))
* complete longitudinal health integration ([d15ce31](https://github.com/T1moooo/BodySense/commit/d15ce31ad2500fa955118a9673418fff4c4e00bd))
* complete longitudinal health integration ([52c2921](https://github.com/T1moooo/BodySense/commit/52c29213c01fbc9ea4cf4524e93db65eb7887232))
* Consultation Agent Platform North-Star (Phase 1-3) ([11ebb09](https://github.com/T1moooo/BodySense/commit/11ebb0913c81f7d80acd14380976dc45a77cf235))
* **deps:** close stream-event drift and export health journey ([f63777c](https://github.com/T1moooo/BodySense/commit/f63777c45dcca650553e23a7309de7bba6e08a70))
* implement architecture-review active plans (W0–W3 + posture) ([617f219](https://github.com/T1moooo/BodySense/commit/617f219ca4564afe01a9a8abea419c43b0db6221))
* integrate streaming runtime events with consultation SSE and learning docs ([4241d7a](https://github.com/T1moooo/BodySense/commit/4241d7ad1c0f4feae82659b2b577766c71a11013))
* knowledge curator and splitter agent platform north-star (phase 1-2) ([bfc445d](https://github.com/T1moooo/BodySense/commit/bfc445d30487c81866074ad7f34d7eabcde94166))
* posture agent platform north-star (phase 1-3) ([455a2c6](https://github.com/T1moooo/BodySense/commit/455a2c6e8ddb69b754a5c19a363a38415b6efc9a))
* Posture/Title/Knowledge Agent Platform North-Star (Phase 1-3) ([0879864](https://github.com/T1moooo/BodySense/commit/0879864a6e3d77746ac0e843b0fa4092c0784120))
* title agent platform north-star (phase 1-2) ([c5203b3](https://github.com/T1moooo/BodySense/commit/c5203b3e3c1189235464caf31ca0bff014c65ca0))
* **web:** activate journey CTAs and consultation resume UX ([1d5964d](https://github.com/T1moooo/BodySense/commit/1d5964d0e39981278595a6aadbc69cb9b8638f03))
* **web:** add unified API URL helper and safe JSON parsing ([10843d3](https://github.com/T1moooo/BodySense/commit/10843d3cf6de7a8b8f58a2b2448f0d754013b6fa))
* **web:** establish desktop workbench foundation ([690b508](https://github.com/T1moooo/BodySense/commit/690b5083a3654520f5213f868d0e1702d4392df0))
* **web:** establish desktop workbench foundation ([2ed1f6c](https://github.com/T1moooo/BodySense/commit/2ed1f6c2fe9fb52a968dbe7771e265148ea90599))
* **web:** introduce resizable consultation workbench ([b2c27a1](https://github.com/T1moooo/BodySense/commit/b2c27a166d5e0073a406cb3d2a1c4c28d9e38fbc))
* **web:** introduce resizable consultation workbench ([a4db22f](https://github.com/T1moooo/BodySense/commit/a4db22f2ead2432aa33d2561e71db5c4134e9f8f))
* **web:** show posture analysis status and results in onboarding ([7d363b4](https://github.com/T1moooo/BodySense/commit/7d363b43ee48b7168714aa1d5fb1ce90614ebdb0))
* 体态照片 AI 分析 (Phase 1 VLM MVP) ([94380e4](https://github.com/T1moooo/BodySense/commit/94380e42d1de2d54b4ce8e4605bcc1f884a3bced))


### Bug Fixes

* **ai:** catch NoAvailableProviderError in fallback loop ([b1d1c04](https://github.com/T1moooo/BodySense/commit/b1d1c045c98e8a010f5b2223332ed76e996d714a))
* **ai:** fail fast when postgres checkpointer is unavailable ([d9c3e43](https://github.com/T1moooo/BodySense/commit/d9c3e43c0b80bd80f9b19763cca0a84fdbaefe6b))
* **ai:** resolve ruff line-length and import-order failures ([405f792](https://github.com/T1moooo/BodySense/commit/405f7921d222c2fbfd2784ae05a91437b3bb9319))
* **ai:** ruff E501 and unused import cleanup for posture/title/knowledge ([a3ba0ce](https://github.com/T1moooo/BodySense/commit/a3ba0ceeaffced6f5f0e4e95b5a4ce73af18cd43))
* **api/ai:** address assessment north-star review findings (S1/I1-I7/A1-A6) ([2cfbd65](https://github.com/T1moooo/BodySense/commit/2cfbd650daf211dc42898622d7f7bd1a118d9c44))
* **api:** migration 000046 index uses started_at (runs has no created_at) ([1974992](https://github.com/T1moooo/BodySense/commit/1974992e6197110d70566fafcd5a8b38dccb5b77))
* **docker:** remove all VITE_API_BASE_URL config to fix double /api prefix ([258a29c](https://github.com/T1moooo/BodySense/commit/258a29c3dd48bd34a2d4f76aeb328722118dcfe5))
* **docker:** remove hardcoded VITE_API_BASE_URL=/api from deploy workflows ([a4bc7d0](https://github.com/T1moooo/BodySense/commit/a4bc7d0783e2f701557b4484afec17042ca7ffaa))
* **docker:** remove hardcoded VITE_API_BASE_URL=/api from web Dockerfile ([334669f](https://github.com/T1moooo/BodySense/commit/334669f604d6f1a0aa0657e479922c28e0ddad48))
* **docker:** remove VITE_API_BASE_URL=/api from DO config to fix double /api prefix ([ffac671](https://github.com/T1moooo/BodySense/commit/ffac67103339049f40a0dec783dd1a983948f076))
* **web:** unify API URL construction and fix double /api prefix bug ([40c7764](https://github.com/T1moooo/BodySense/commit/40c77647621fb3c6a94dd303cc6022917fe36ce8))
* **web:** 登录/注册表单及错误提示改为中文 ([a201571](https://github.com/T1moooo/BodySense/commit/a2015716937bd1e2d146c57e4f31c207574cba98))

## [0.3.0](https://github.com/T1moooo/BodySense/compare/v0.2.0...v0.3.0) (2026-07-09)


### Features

* **ai,api:** add FaithfulnessPolicy, wire governance observe-only persistence ([afdf961](https://github.com/T1moooo/BodySense/commit/afdf961c850a946e4891b0eeec0ec259c862d4f6))
* **ai,api:** wire ask_user HITL end-to-end path ([4624443](https://github.com/T1moooo/BodySense/commit/4624443f0d5653289cce62abc4be9bb0a98e0ff4))
* **ai:** add agent orchestrator skeleton and update knowledge library policies ([5b6f460](https://github.com/T1moooo/BodySense/commit/5b6f460607dd4a372675a0992d3291f21c41fd47))
* **ai:** add agent runtime module and update consultation tools ([6402795](https://github.com/T1moooo/BodySense/commit/6402795c3daaf197687bb29cb84581166851740d))
* **ai:** add curated specifications and ingest pelvic, shoulder, and thoracic mobility data ([c4e32e4](https://github.com/T1moooo/BodySense/commit/c4e32e492a336b6f677cfea39c4d494c9cd2fbc7))
* **ai:** add ingest_generated_source script to ingest raw generated packs ([8497306](https://github.com/T1moooo/BodySense/commit/849730682b5520ed9400fa406ca4b44b6204484d))
* **ai:** enhance ask_user tool with question normalization and smart options ([212955c](https://github.com/T1moooo/BodySense/commit/212955ccde5f4d5c2f4f462a5a1c11f1ed99bb29))
* **ai:** implement multi-provider router, circuit breaker, and unified AIService ([c2f02c8](https://github.com/T1moooo/BodySense/commit/c2f02c8e55c80a121cdf46a7c8bf72a8b9b3c4d9))
* **ai:** support multimodal assessment and chat stream events ([e7c44f8](https://github.com/T1moooo/BodySense/commit/e7c44f8835c831517594e30a4d4f7ad1fd416d65))
* **ai:** tighten ask_user interruption rules ([188e7fa](https://github.com/T1moooo/BodySense/commit/188e7fa85d4400e73cdf1b785b3be5c2fa97042c))
* **api,web,ai:** add service tests, AskUserCard error state, timed_out transition ([4fd2e32](https://github.com/T1moooo/BodySense/commit/4fd2e32078336457c43cdb534335f653774183b1))
* **api:** add assistant parts builder for structured message construction ([3cec4e4](https://github.com/T1moooo/BodySense/commit/3cec4e49e4bb7b5923ed03b780382b17cc7c6698))
* **api:** add consultation runtime module and refactor ai_client ([94f801f](https://github.com/T1moooo/BodySense/commit/94f801fa481213b6dee62bb4d3ffa31b19e614b5))
* **api:** add database migrations for user uploads and knowledge library ([0253042](https://github.com/T1moooo/BodySense/commit/025304237e747d809c845d55c82813dfd339dff0))
* **api:** add knowledge publication batches, chat runtime, and job runtime enhancements ([caffc99](https://github.com/T1moooo/BodySense/commit/caffc995fe04f07c5e234a56345a35e93d0bd3bf))
* **api:** add KnowledgePublication repository ([b41a438](https://github.com/T1moooo/BodySense/commit/b41a4388ca8a8dada7f9dc978340a15361d7409c))
* **api:** add message metadata and improve interaction handling ([b3c418c](https://github.com/T1moooo/BodySense/commit/b3c418cc7f9668b5d41e2517b7bab6aecd0fd184))
* **api:** converge consultation run runtime ([1b8069b](https://github.com/T1moooo/BodySense/commit/1b8069b41517f9a42f05cc1fbe0afbffb062dbfb))
* **api:** implement database migrations, repositories, and Gin handlers for session redesign ([154dd28](https://github.com/T1moooo/BodySense/commit/154dd289e605d2ae1015fb5d1a94c079ece42071))
* **api:** implement multimodal assessment, log adjustments, and chat SSE flow ([9ac61b6](https://github.com/T1moooo/BodySense/commit/9ac61b6e48b0d56727c2f9638adccb13e9cf9244))
* **api:** persist structured health features ([6f0e7cf](https://github.com/T1moooo/BodySense/commit/6f0e7cf1f5fe4d4a177554c24084b862025ffcab))
* complete ask_user runtime flow and structured health features ([af42132](https://github.com/T1moooo/BodySense/commit/af42132d335762e2f6e4877e269729d77ceaf384))
* converge consultation run architecture ([e0aacb0](https://github.com/T1moooo/BodySense/commit/e0aacb0495e786805689a810515b2fcd28c4c825))
* **docker:** add DigitalOcean production compose ([b286828](https://github.com/T1moooo/BodySense/commit/b286828f0294854e9417b6fc1bf486520743468b))
* **docker:** add resource limits and logging for DO compose ([5f016e1](https://github.com/T1moooo/BodySense/commit/5f016e16f7a177467b1514757de586eb9a172859))
* **web:** add streaming assistant turn and tool call components ([8910c0f](https://github.com/T1moooo/BodySense/commit/8910c0f1d7a4467317ffd9cf409b634fcc56d7a0))
* **web:** consume consultation run streams ([a87b56c](https://github.com/T1moooo/BodySense/commit/a87b56cbfbab35116bdd79e56f1a9a326d6fe772))
* **web:** enhance ConsultationPage with pending interactions and session management ([f877450](https://github.com/T1moooo/BodySense/commit/f877450869a37e7132d531cf6589c1c3ab1ddaf3))
* **web:** extend consultation types and service for new architecture ([32bdd06](https://github.com/T1moooo/BodySense/commit/32bdd06e500326f64a9f22a11a7e596105734d47))
* **web:** integrate @assistant-ui/react runtime, add shadcn/ui components, and redesign sidebar ([39717c7](https://github.com/T1moooo/BodySense/commit/39717c7c820c88d6edbe827d338f79d0dd039da2))
* **web:** integrate onboarding upload, diagnostic dual-sync, and immersive timer player ([6645f19](https://github.com/T1moooo/BodySense/commit/6645f195404af6c4150b9c3f2d33e87de861990a))
* **web:** replay ask_user interactions in thread timeline ([747845a](https://github.com/T1moooo/BodySense/commit/747845add68f89eeac0cb42dc8472c56af3ce48e))
* **web:** surface health features in consultation panel ([251ecbf](https://github.com/T1moooo/BodySense/commit/251ecbf09e1baa6dd04ba328933935aa38582821))
* **web:** update onboarding layouts and profile view/edit components ([47b541f](https://github.com/T1moooo/BodySense/commit/47b541f032484d86cb92ea167c1b3e7ae48a82bd))


### Bug Fixes

* **ai:** add tool call deduplication and improve interaction handling ([f937773](https://github.com/T1moooo/BodySense/commit/f937773516fa9e218c909fde341ecc9aa950fc88))
* **ai:** pool consultation checkpointer connections ([76ae4e1](https://github.com/T1moooo/BodySense/commit/76ae4e142e8dae8d5657987ac9dd6cb8bdcc8178))
* **ai:** prevent duplicate tool call emissions and add tool_call_id to events ([bd05fd1](https://github.com/T1moooo/BodySense/commit/bd05fd18cac29b32bcd4294babd096406ff09b17))
* **ai:** update knowledge ingestion scripts and ASR pipeline ([5795568](https://github.com/T1moooo/BodySense/commit/57955680a7c4091465aba3470688489b628a4ac8))
* **ai:** 修复 ruff E501 过长行报错 ([362352f](https://github.com/T1moooo/BodySense/commit/362352f5feae1b8a2b22068ec8a8c44fc00775f2))
* **api,ai,web:** unify stream event model across Go, Python, and TypeScript ([d84103e](https://github.com/T1moooo/BodySense/commit/d84103e32e42985545ebffd64a6a63a38c65259e))
* **api,ai:** quality fixes for tool error detection, governance output, and resume handler ([774e434](https://github.com/T1moooo/BodySense/commit/774e434e820e873ed9b8b6e22b80cddba8e59bc9))
* **api,web:** update chat handler and remove stale package-lock.json ([ce74641](https://github.com/T1moooo/BodySense/commit/ce7464196cf46bb7fa43305c3580f977a3cff591))
* **api,web:** wire resume to send_message flow, add ASK_USER_ENABLED feature flag ([804808e](https://github.com/T1moooo/BodySense/commit/804808e1151e12ca7862c01955a376d10e48824f))
* **api:** add job idempotency fields, fix OCR TOCTOU race, add waiting_user status ([9dfc0a3](https://github.com/T1moooo/BodySense/commit/9dfc0a3c7c32ad42de595501aee78528fdf20ad3))
* **api:** check and reuse existing empty consultation session ([3debc8f](https://github.com/T1moooo/BodySense/commit/3debc8f19caa7673e6d75625ea8a1416efb4841b))
* **api:** complete implementation audit gaps ([8aa6bf1](https://github.com/T1moooo/BodySense/commit/8aa6bf10cbca134c2d85fcc19618b8d6bf07362d))
* **api:** correct knowledge lifecycle schema gaps from 07a plan ([63043b0](https://github.com/T1moooo/BodySense/commit/63043b01eb746a788c73197b8a367064b1c7fc51))
* **api:** support managed database TLS connections ([0f9c697](https://github.com/T1moooo/BodySense/commit/0f9c697f5a13545df0d16de9e84a2833b9eafd97))
* **deps:** add interaction_id to stream event JSON schema ([c14a780](https://github.com/T1moooo/BodySense/commit/c14a780a97030b4b175dd58184d8750c7947ae6f))
* make workflow checks pass ([c7b3032](https://github.com/T1moooo/BodySense/commit/c7b30327beed391a393b0891476142c6262c781c))
* **web:** prevent duplicate session creation, fix SSE parsing and sync symptom list ([14ddd5b](https://github.com/T1moooo/BodySense/commit/14ddd5bce6769f9374453963bb521bf1c771cd0b))
* **web:** rename Button.tsx to button.tsx to match import casing ([a82a6f4](https://github.com/T1moooo/BodySense/commit/a82a6f4f07c56c89b0e1a87a98e77ca7fe26c18a))
* **web:** resolve CI lint failures ([80eb884](https://github.com/T1moooo/BodySense/commit/80eb88406eb1db73eb82405c81e6b62f9fdf1a2a))
* **web:** update upload step and training page components ([b4a2566](https://github.com/T1moooo/BodySense/commit/b4a2566d1ba4342d3bf807ae70e6cf73de076d7c))
* **web:** use PascalCase Button import to match component filename ([eabfd44](https://github.com/T1moooo/BodySense/commit/eabfd44607006dbc5649ecc08f3b4f4a0068eec5))
* **web:** 修复 ESLint 报错 ([a39ab2e](https://github.com/T1moooo/BodySense/commit/a39ab2ee313d7f316f2453ea66828d9f0c8556e5))


### Performance Improvements

* **web:** 优化认证 store 性能 ([cee8cdc](https://github.com/T1moooo/BodySense/commit/cee8cdc2df2be05da2485723a5fd8504a00d0c3d))
* **web:** 优化问诊工作台渲染性能 ([7932826](https://github.com/T1moooo/BodySense/commit/79328263a3884325499551702b2e96160a032df1))
* **web:** 优化问诊线程查询 hook 性能 ([ffeeac9](https://github.com/T1moooo/BodySense/commit/ffeeac94d69baed29242406f705275059ffc0f9d))
* **web:** 优化问诊页面渲染性能 ([26373b3](https://github.com/T1moooo/BodySense/commit/26373b39c1ac05ee6d9bb55a1cc7608826c10c35))
* **web:** 添加懒加载和骨架屏优化架构 ([2cad847](https://github.com/T1moooo/BodySense/commit/2cad847fde340e737bf68c58d66feaebd83d8690))
* 设置默认的新绘画语义为 `/` ， 同时保留`/new` 。 ([4841713](https://github.com/T1moooo/BodySense/commit/48417135ed07bf9e6f6bce1a36a627fa722f79ec))

## [0.2.0](https://github.com/T1moooo/BodySense/compare/v0.1.0...v0.2.0) (2026-06-23)


### Features

* **ai,api,web:** implement consultation chat with LLM streaming and symptom extraction ([0b7b9f1](https://github.com/T1moooo/BodySense/commit/0b7b9f1874b574c48c8a9a69e2bd251026c4c6ef))
* **ai,api,web:** implement diagnosis analysis and treatment plan generation ([c742594](https://github.com/T1moooo/BodySense/commit/c7425948b5d662f1f0c8bb54f83426598f168186))
* **ai,api,web:** implement health assessment report generation ([9b6f1e2](https://github.com/T1moooo/BodySense/commit/9b6f1e24634583fc9045496b96e8daa93034970b))
* **ai,api,web:** implement progress tracking and reassessment ([2f6283e](https://github.com/T1moooo/BodySense/commit/2f6283e0059907e12e896a65a965bfa067232961))
* **ai,api,web:** Issue [#10](https://github.com/T1moooo/BodySense/issues/10) - 可能性分析 + 方案生成 ([96d563c](https://github.com/T1moooo/BodySense/commit/96d563cb859c7667fed780a304dfe95472a5beb5))
* **ai,api,web:** Issue [#12](https://github.com/T1moooo/BodySense/issues/12) - 进度追踪 + 阶段性复评 ([743b8f5](https://github.com/T1moooo/BodySense/commit/743b8f5f3c47c8b00627cbb7c1e3d26172b6fd6f))
* **ai,api,web:** Issue [#6](https://github.com/T1moooo/BodySense/issues/6) - 咨询工作台 LLM 流式聊天 + 症状提取 ([f1308f6](https://github.com/T1moooo/BodySense/commit/f1308f6cc2534e892ef110d5bac59641dc864ccb))
* **ai,api,web:** Issue [#7](https://github.com/T1moooo/BodySense/issues/7) - 健康评估报告生成 ([3d0f8b3](https://github.com/T1moooo/BodySense/commit/3d0f8b3fbe2984dcdf8ca0d4675fde2886aa0f54))
* **ai,api:** implement issue-13 forward head knowledge pilot with RAG pipeline ([2b5d645](https://github.com/T1moooo/BodySense/commit/2b5d64595ec3aa80fc015eae488ed111bd4b3cde))
* **ai,api:** issue-13 forward head knowledge pilot with RAG pipeline ([2c417ba](https://github.com/T1moooo/BodySense/commit/2c417bac7a78bcc5da906135aa3c4a4a785efc3e))
* **ai:** add mimo model support for embedding and reranking ([0698291](https://github.com/T1moooo/BodySense/commit/069829100300bd8a06cd3ae56a6429be346682d5))
* **ai:** implement RAG infrastructure with pgvector, embedding, and semantic retrieval ([3b9b1e8](https://github.com/T1moooo/BodySense/commit/3b9b1e868fe6144079ae041f35319c5db09b758e)), closes [#3](https://github.com/T1moooo/BodySense/issues/3)
* **api,web:** implement training plan and daily check-in ([16e7daa](https://github.com/T1moooo/BodySense/commit/16e7daadd408b86a4e955cb16539871fecebb493))
* **api,web:** implement user auth with JWT and login/register UI ([19f6306](https://github.com/T1moooo/BodySense/commit/19f6306c742ad38063d4c989c0dd1b45ba773faa))
* **api,web:** Issue [#11](https://github.com/T1moooo/BodySense/issues/11) - 训练计划生成 + 每日打卡 ([55d962c](https://github.com/T1moooo/BodySense/commit/55d962c639637f8a082037442b9a8ed63d829b70))
* **docker:** 搭建开发环境基础设施 ([0b96db4](https://github.com/T1moooo/BodySense/commit/0b96db41ac98f03ea312ce4ab99cd313f0e5932a))
* issue5,ocr + upload... ([3c64431](https://github.com/T1moooo/BodySense/commit/3c64431f963b8b26a41947e655b86c1cca8cef7e))
* issue5,ocr + upload... ([c8d2438](https://github.com/T1moooo/BodySense/commit/c8d243883e861297ea70e7834ccd44b1295b48e6))
* **profile:** implement body info collection and profile management ([92a5430](https://github.com/T1moooo/BodySense/commit/92a54303c97a95ce97f3e3c4f94f9d250dbf1620))
* **profile:** implement body info collection and profile management ([6b56432](https://github.com/T1moooo/BodySense/commit/6b56432587f68c00154ec8fb1c517a214c4981fb)), closes [#4](https://github.com/T1moooo/BodySense/issues/4)
* **web:** implement info panel with body visualization ([755e32b](https://github.com/T1moooo/BodySense/commit/755e32b186963372d77b8b0b32449bb843bd43e5))
* **web:** implement session history page ([311a4f1](https://github.com/T1moooo/BodySense/commit/311a4f14ea2ecf5be1e786785b147e2682dfdac4))
* **web:** Issue [#8](https://github.com/T1moooo/BodySense/issues/8) - 信息面板 + 身体可视化 ([424e22a](https://github.com/T1moooo/BodySense/commit/424e22aa371f900d3abccfcd2a34f98b1152ee3a))
* **web:** Issue [#9](https://github.com/T1moooo/BodySense/issues/9) - 会话保存 + 历史记录 ([f5c3a4b](https://github.com/T1moooo/BodySense/commit/f5c3a4b3d98007790c15c756b8237f8d9d58a9de))
* 搭建开发基础设施并实现用户认证 + JWT 鉴权 ([b40cd47](https://github.com/T1moooo/BodySense/commit/b40cd47b8ac0fa95b5850d518e59237859f4921c))


### Bug Fixes

* **ai,api:** 修复测试、lint 错误、迁移冲突并优化镜像 ([1886aff](https://github.com/T1moooo/BodySense/commit/1886aff7439bc01864c97c4109defc564301c4d1))
* **api,ai:** resolve migration numbering conflict and align embedding dimension to 1536 ([648ee1f](https://github.com/T1moooo/BodySense/commit/648ee1f5c4aa82ba419efe7755444e28928057de))
* resolve Windows asyncio event loop issue and switch to sync psycopg ([d8d9b43](https://github.com/T1moooo/BodySense/commit/d8d9b43ac866dd1d63761891adc03926de12cf1b))
