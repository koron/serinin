# test servers for serinin

## feature

クエリ文字列で `sleep.{id}` にduration string (例: `0.5s`)を渡すと、該当するID
のサーバーがレスポンスを返すのにその時間待つ。serinin側のエンドポイントごとのタ
イムアウト設定が機能しているかどうかに利用できる。
