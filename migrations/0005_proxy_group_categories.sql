-- 0005_proxy_group_categories.sql
--
-- M-RULE 扩展：代理组（Proxy Group）持久化层。每行描述用户自定义的一个
-- proxies group，最终在 Clash YAML 的 `proxy-groups:` 块里展开。
--
-- type 取值与 mihomo 一致（select / url-test / fallback / load-balance / relay）。
-- member_proxies / member_groups 为 JSON 文本：前者列出节点名 / 内置出口
-- （DIRECT / REJECT），后者引用其它组的 id 形成嵌套（前端在装配 YAML 时把
-- group id 翻译成 group name）。filter 用作正则筛选节点池，include_all=1
-- 时表示"先全量再过滤"，否则只取 member_proxies 显式列出的。

CREATE TABLE IF NOT EXISTS proxy_group_categories (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    type          TEXT NOT NULL CHECK (type IN ('select', 'url-test', 'fallback', 'load-balance', 'relay')),
    icon          TEXT,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    test_url      TEXT,
    test_interval INTEGER DEFAULT 300,
    filter        TEXT,
    include_all   INTEGER NOT NULL DEFAULT 0,
    member_proxies TEXT,
    member_groups  TEXT,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_proxy_group_categories_user_sort
    ON proxy_group_categories(user_id, sort_order);
