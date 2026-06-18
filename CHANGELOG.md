# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-18

### Bug Fixes

- [9e168a7](https://github.com/colinwilliams91/total-recall/commit/9e168a71203d18bea0c86c741a4e7fb08a883033): tui hangs bug, needs cleanning out logs
- [748102f](https://github.com/colinwilliams91/total-recall/commit/748102f20b2a9bda878f94924726f78a950659dc) **(cli)**: daemon terminal should not prompt TR question, all choices from LLM should render (not cut off at 3), BubbleTea needs to exit to CWD after user choice submission (WIP)
- [fd8b7f0](https://github.com/colinwilliams91/total-recall/commit/fd8b7f09f999d4cfda82cdff0f77deaef49e20f0) **(tui)**: exit BubbleTea back to terminal cwd on ask and on served commit (the latter is 90% fixed, requires any key press after but looks clean enough for mvp)
- [2a798ca](https://github.com/colinwilliams91/total-recall/commit/2a798ca2789a1dac60f71fea7e801e19a71cbf8d) **(hooks)**: add sentinel comment like other hooks to post-commit writer, use os.Executable instead of hardcoded which total-recall which is unreliable, this new impl is PATH agnostic as it identifies the running binary absolute path
- [78ee959](https://github.com/colinwilliams91/total-recall/commit/78ee959ff258cb67cacde5535df6f74b598a9d5d) **(choices)**: fisher-yates shuffle choices downstream from external LLM recall question synthesis (always responds correct answer at index 0) -- this randomizes correct answer index per question instead of static (required for answer placement determinism)
- [7df305d](https://github.com/colinwilliams91/total-recall/commit/7df305d539a34153bdcd6a8ddf8bd06c00763cd0) **(chore)**: warn users on any unresolvable env value on server start
- [adc7976](https://github.com/colinwilliams91/total-recall/commit/adc79769c4e13578488add44613781522be6e19d) **(e2e)**: during manual testing we surfaced some env, process, and AI response (max token truncation and validation failure) issues
- [c10df19](https://github.com/colinwilliams91/total-recall/commit/c10df197dd867efd6ddd4eb316e9830130f24000) **(py)**: replace multiline string with single line string for cross platform py -> bash argv receipt, fixes goto error on git push

### Documentation

- [49cf747](https://github.com/colinwilliams91/total-recall/commit/49cf747c8e820d418d23a944083e0fea25513fb0) **(schema)**: questions table schema v2
- [fe12e42](https://github.com/colinwilliams91/total-recall/commit/fe12e4213eede712d3568c9fc706631020278758) **(opsx4A)**: phase 4A mcp-delivery
- [959a2cf](https://github.com/colinwilliams91/total-recall/commit/959a2cf23b2a6cc2c00f72fe20d41b23d05948c0) **(e2e)**: testing md full cross platform
- [ad77ac5](https://github.com/colinwilliams91/total-recall/commit/ad77ac5d690bea0c1e7a1e01177a672277ff890e) **(opsx3)**: phase 3 completion docs, needs e2e and regression testing
- [0765b47](https://github.com/colinwilliams91/total-recall/commit/0765b47fb427a4ac907527515a7801f3bd875434) **(opsx3)**: phase 3 documentation, spec, design, etc, verified and greenlit
- [807a8a0](https://github.com/colinwilliams91/total-recall/commit/807a8a0883805894cea3b13aa8eea78abff78d1f) **(mm)**: update mermaid ignore to include openspec/archives for updating diags
- [54d38c4](https://github.com/colinwilliams91/total-recall/commit/54d38c480218ff5f5e7436bb371759a86f83cf03) **(readme)**: clean up redundant information in other md files, align how to use and --help docs
- [c6dfb66](https://github.com/colinwilliams91/total-recall/commit/c6dfb667c4bd3093258e8c2f53c1ff9d027d48e4) **(todo)**: ai provider api key needs dynamic
- [52f7340](https://github.com/colinwilliams91/total-recall/commit/52f73407f6b8702ae47def80eae1162f191fc462): readme contributing how to, build run test
- [a791d63](https://github.com/colinwilliams91/total-recall/commit/a791d6387b65830784e3a7acbf2f97c975559808): single http server/daemon port, 2x prefixed API urls (internal)
- [a2be01a](https://github.com/colinwilliams91/total-recall/commit/a2be01a70d17b26750ac5360c7ccc17e480c5c50): todos pre v1
- [3f9b302](https://github.com/colinwilliams91/total-recall/commit/3f9b3025058fb1fbed8150b5a0133c36254ff7a0) **(skill)**: quiz generator frontmatter yaml
- [506c770](https://github.com/colinwilliams91/total-recall/commit/506c770083d6b98ba87413ac6b95a4a880472338) **(brief)**: initial human pitch, ehnanced
- [69a49e4](https://github.com/colinwilliams91/total-recall/commit/69a49e45116c61112b1550af230c6665b4095836) **(submodule)**: git_hooks in event_monitor primary md
- [de3288c](https://github.com/colinwilliams91/total-recall/commit/de3288c55c5790e533e89c8a375c38f18791af62) **(fix)**: fig 6 and 11 charts broken
- [9948724](https://github.com/colinwilliams91/total-recall/commit/9948724ee0821330e0e9a01dd0910e314626390b) **(import)**: v3 from obsidian local vault, docs should be finished and ready for opsx handoff
- [38d93d8](https://github.com/colinwilliams91/total-recall/commit/38d93d8bc6e839c28161081e70fff9bd2180918f) **(readme)**: hone readme, reimport images for charts and logo
- [2bb649b](https://github.com/colinwilliams91/total-recall/commit/2bb649b90257dff115eb5f6e3bb11cf9d7f41cfa) **(readme)**: v1 readme; elevator pitch, research examples, philosophy, roadmap

### Features

- [508695c](https://github.com/colinwilliams91/total-recall/commit/508695c18e534b20b56fda663a31a96e6e08bf84) **(release)**: init release cross platform release system, CI, changelog, semver, conventional commit grep, ignored bins
- [20a7f33](https://github.com/colinwilliams91/total-recall/commit/20a7f33fe06f97329aeb9ed494171420e69953ec) **(archive)**: opsx phase 4A complete, full E2E green
- [5bb3e04](https://github.com/colinwilliams91/total-recall/commit/5bb3e04944e24e043aa7092ec3024ac90111f02e) **(ask)**: report daemon not running when true
- [8369b63](https://github.com/colinwilliams91/total-recall/commit/8369b635d107dc1b79b14294dfd07e994147487f) **(opsx)**: propose 4C answer delivery feedback mcp and terminal routes
- [04967ed](https://github.com/colinwilliams91/total-recall/commit/04967ed7e86feb7a9b56d8fde8363aaf8e272e01) **(queue)**: dequeue on empty questions should exit with notification
- [7bda568](https://github.com/colinwilliams91/total-recall/commit/7bda568517cd47c990ea8ac9d49aae1292ed8db6) **(opsx4A)**: phase 4A mcp-delivery v1 finish, E2E in process, refinement in process
- [b4c55cc](https://github.com/colinwilliams91/total-recall/commit/b4c55ccc7822466670df2d0b7742c966e9cf968f) **(opsx)**: phase 4A multiple iterations of explore -> proposal commit
- [c83c076](https://github.com/colinwilliams91/total-recall/commit/c83c0766300c109f2f05392bb6a414991d27a9b8) **(opsx3)**: spec phase 3 impl
- [201c004](https://github.com/colinwilliams91/total-recall/commit/201c004b62af6b420699b30ea57b67895a3a94b3) **(mm)**: mermaid ignore and smart mermaid update excludes
- [8f9e5b2](https://github.com/colinwilliams91/total-recall/commit/8f9e5b263a32a023b1aeb0317597d27041391e2e) **(mm)**: rm health status flow from arc event diag
- [75a4547](https://github.com/colinwilliams91/total-recall/commit/75a4547fcabb6b6d182d33447c514d172203b8ae) **(hooks)**: explicitly log skipping recall check on daemon not running and git event (respectively)
- [227358d](https://github.com/colinwilliams91/total-recall/commit/227358d95df8207c9cff12df4aa8527522fa9670) **(p2)**: manual E2E passing 1-8 (binary build also passes), daemon runs, hooks are installed, if hooks exists tr hook wraps existing, config scopes correctly
- [3ffecbe](https://github.com/colinwilliams91/total-recall/commit/3ffecbec9971b5837c2d84f88c49dca8e76ffb44) **(config)**: implement config-architecture (23/23 tasks)
- [adaff74](https://github.com/colinwilliams91/total-recall/commit/adaff74503ae5a9cb859193ffd0d539170efe8fd): scaffold Go project structure (phase 00)
- [3585fad](https://github.com/colinwilliams91/total-recall/commit/3585fad85abdfb445e5f87e13805c0b35ef04a23) **(openspec)**: add huh to phase-00 CLI framework decision
- [43effab](https://github.com/colinwilliams91/total-recall/commit/43effab2720b473a4d6e77bea59cb42d22520508) **(openspec)**: add phase-00-scaffolding change proposal
- [13bc8ef](https://github.com/colinwilliams91/total-recall/commit/13bc8ef46f57c333a42c21124f9d7ce0265dfdce): init v1 openspec/config.yaml
- [43560f9](https://github.com/colinwilliams91/total-recall/commit/43560f904e0865d34fd42b21e40d3265b7507a86): init opsx
- [2c91874](https://github.com/colinwilliams91/total-recall/commit/2c918746e5d5ba9127b6f21891e95b0d05af5662) **(skills)**: destructure skill into asset/prompt to inject by go runtime code, update stale MCP adapter fn plumbing and event monitor stale artifacts

### Refactoring

- [1fc405b](https://github.com/colinwilliams91/total-recall/commit/1fc405bebfa10d478b5a632c96f6c6a2a556f582) **(agent)**: replace opsx .github w/ .opencode equivalent (tools)

### Testing

- [0906ad6](https://github.com/colinwilliams91/total-recall/commit/0906ad60d7690ef6e670600311c2d93049f36ade) **(daemon)**: adapter and ask, e2e testing + design and spec docs drift update for prev commit
- [b54d440](https://github.com/colinwilliams91/total-recall/commit/b54d440019938e520be57e9d80d093f99c675ada) **(ask)**: report daemon not running tests adn docs for drift
- [c30e3b4](https://github.com/colinwilliams91/total-recall/commit/c30e3b4275b3bdf6caf98022624847ab5a9b289d) **(queue)**: questions dequeue on empty test and docs update
- [8187e0a](https://github.com/colinwilliams91/total-recall/commit/8187e0a6cc6527a43d2dfbe90a0dc008ed0d525a) **(hook)**: push test green, rebuild worked, deleting test file
- [7bad668](https://github.com/colinwilliams91/total-recall/commit/7bad668948e8fe716eda203f11582a91bc95a169) **(hook)**: push rebuilt after argv line fix
- [de29e35](https://github.com/colinwilliams91/total-recall/commit/de29e3565ac6c64efd85cfda79bc5f18a3d0633a) **(e2e)**: update hook invocation steps in e2e doc
- [9ef3acf](https://github.com/colinwilliams91/total-recall/commit/9ef3acfee89b3ee62045dbd3df037dfc8471a253) **(50%)**: huh form works, ~/.tr/config.yaml generated, also persists prev defined on init call, serve working, init outside repo working, --quiet flag works, needs --show, repo override, and hide api key values (on show) tests

### Arch

- [218d3df](https://github.com/colinwilliams91/total-recall/commit/218d3dff25ade720bd13af4ee7b0441fc0398320): daemon required; defer transient mode indefinitely

### Design

- [57d65ed](https://github.com/colinwilliams91/total-recall/commit/57d65ed2dae8461e41762883c562a3e3441da0a9): config-architecture change

### Openspec

- [cf50764](https://github.com/colinwilliams91/total-recall/commit/cf5076416113b869ea8e336794469e1e2ca0ee99) **(config-architecture)**: align docs — opt-in at init, close --quiet decision

### Security

- [8d6c9e7](https://github.com/colinwilliams91/total-recall/commit/8d6c9e7b44e14e47702c592055607765069d9230): escalate .tr.yaml credential warning; flag P0 for hooks phase
<!-- generated by git-cliff -->
