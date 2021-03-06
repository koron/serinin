# Design of serinin

## What is serinin

serinin は受けたリクエストをバックエンドの複数のエンドポイントに転送する一種のリバースプロキシで、
それらエンドポイントが返すレスポンスのマルチプレクサとして機能します。

serinin はリクエスト毎にリクエストIDを発行します。
serinin はエンドポイントから非同期にレスポンスを取得し redis や memcached 等のストアに格納します。
また指定時間(初期値:200ms)以内に応答しなかったエンドポイントのレスポンスは格納しません。
また格納したレスポンスは一定時間(`"expire_in"`)で消えるように設定します。

serinin自身のレスポンスではリクエストIDと各エンドポイントの名前を応答します。

クライアントは任意の時間経過後に、リクエストID(およびエンドポイント名)を用いて各ストアに所定の方法でレスポンスの本文を問い合わせます。

各レスポンスの長さに制限はありませんが数十バイトから長くとも数Kバイトを想定います。

## How to access responses

クライアントがレスポンスにアクセスする方法はストア毎に異なります。

### Redis store

redis ストアにおいてはレスポンスはリクエストIDをキーにしてハッシュとして格納されます。

ハッシュ内のフィールド名と意味は以下の通りです。

* `_id` - リクエストID
* `_method` - リクエストメソッド
* `_url` - リクエストURL(パス)
* `{エンドポイント名}` - 各エンドポイントが返したレスポンス本文

クライアントがアクセスする際は [`HGETALL {リクエストID}`](https://redis.io/commands/hgetall) コマンドを用いる想定です。

### Memcache store

memcache ストアにおいてはレスポンスはリクエストIDおよびエンドポイント名をキーにして格納されます。

キーの定義と対応する値の意味は以下の通りです。

* `{リエクエストID}` - リクエストの情報が JSON Object で格納される:

    * `_id` - リクエストID
    * `_method` - リクエストメソッド
    * `_url` - リクエストURL(パス)

* `{リクエストID}.{エンドポイント名}` - 各エンドポイントが返したレスポンス本文

## How to work serinin

serinin はクライアントからリクエストを受けると以下のように動作します。
このリクエストはリソースが許す限り並列に処理しますが、数が多くなると輻輳を起こしパフォーマンスが低下するため `-handler {同時リクエスト処理数}` オプションで制限することを推奨します。

1. リクエストIDを生成する (UUIDv4)
2. ストアにリクエストIDを含むリクエストの情報を保存する 
3. ジョブキューにエンドポイント毎の取得&格納ジョブを投入する
4. リクエストIDおよびエンドポイント名を含むレスポンスをクライアントに返す

    レスポンスはJSON Objectで以下のプロパティを持つ

    * `request_id` - 文字列。リクエストID
    * `endpoints` - 文字列の配列。全てのエンドポイント名

ジョブキューは標準ではリソースの許す限り並列に処理を試みますが、こちらも数が多くなると輻輳を起こしパフォーマンスが低下するため `-worker {同時実行ジョブ数}`
オプションで制限することを推奨します。
「同時実行ジョブ数」の参考値は「同時リクエスト処理数」×「エンドポイント数」です。

取得&格納ジョブはエンドポイントにリクエストを転送し、そのレスポンスをストアに格納します。
転送するリクエストには元リクエストのクエリー文字列が適用されます。
指定時間内にエンドポイントからレスポンスが得られない場合はストアには何も記録しません。
また仮にがレスポンスが得られてもストアでの記録中に指定時間を超過した場合には記録されないことがあります。

### Why?

Goはgoroutineにより気軽に並列処理が可能ですが、コンピューターで利用可能なCPUコア数を大きく超えるgoroutineを起動すると極端にパフォーマンスが悪くなる他、最悪リ
ソース(スレッド数やメモリ等)不足でpanicにより終了してしまいます。
それを避け安定した動作をし続けるためにはhttp.Serverで同時に処理するリクエスト数を制限するなどして、goroutineの数を制限するもしくは厳密に管理する必要があります。

また特別なケースを除きgoroutineの実行時間を可能な限り短く保つことも重要です。
リクエストが定常的に発生する状況下で1つのgoroutineの実行時間が長くなってしまうと、同時に起動されるgoroutine数が多くなり結果として不必要なパフォーマンスの低下を招くことになります。
