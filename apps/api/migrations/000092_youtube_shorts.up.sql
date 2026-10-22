-- YouTube の Shorts の URL から作った投稿かを残す。縦長の動画を縦に表示するために使う。
-- URL 自体は残していないので、すでにある投稿は横のまま（false のまま）。
ALTER TABLE posts ADD COLUMN youtube_shorts boolean NOT NULL DEFAULT false;
