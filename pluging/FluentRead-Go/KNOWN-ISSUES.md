# KNOWN-ISSUES 遗留问题清单

> 纪律（任务书）：P0/P1 必须为空；P2 起允许遗留，但必须注明位置与复现步骤。
> 定级：P0=会让人直接放弃；P1=每次用都遇到且明显影响效率；P2=不影响做事但看着不对。

## 当前状态

P0–P6 出口门禁全部通过（各期 gates.txt 为证）。遗留 1 条，为**环境限制**而非功能缺陷，不影响交付验收：

| 期 | 编号 | 级别 | 位置 | 复现步骤 | 说明 |
| --- | --- | --- | --- | --- | --- |
| P7 | P7-01 | P2（环境限制） | deploy/Dockerfile | 在本沙箱执行 `docker compose -f deploy/docker-compose.yml build app`：拉取 golang/node 基础镜像时直连 Docker Hub 与镜像源（daocloud）均超时（EOF/DeadlineExceeded，留证 docs/acceptance/P7/evidence/docker-build.txt） | 容器级构建/运行验证在本环境无法执行（已按 SKIPPED 留证）。交付物本身完整：Dockerfile（多阶段，基础镜像可经 GO_IMAGE/NODE_IMAGE 构建参数替换）、compose（app 单容器 + litellm 可选 profile）均通过 `docker compose config` 校验；"单容器独立运行 + health 自检"的实质已由本地等价自检证明（同一二进制 + mock 网关，health 全 ok + 翻译冒烟通过，见 docs/acceptance/P7/evidence/health-local.json）。在能访问 registry 的机器上按 deploy/README.md 第 1 节复现即可补跑容器级验证。 |
