(ns csv-parser.core (:require ys.v0))
(ys.v0/init)
(declare
 collection-events
 parse-options
 fail
 scalar-event
 event
 detect-format
 validate-header
 select-format
 mapping-events)
(defn
 parse
 [reader text config environment]
 (let
  [options
   (parse-options config environment)
   format
   (get+ options (quote format))
   format
   (if (= format "auto") (detect-format text) format)
   separator
   (if (= format "tsv") 9 44)]
  (set! (. reader Comma) (go/rune separator))
  (let
   [[rows error] (. reader ReadAll)]
   (when error (throw error))
   (let
    [rows (mapv vec rows) header (get+ options (quote header))]
    (when (and header (seq rows)) (validate-header (get+ rows 0)))
    (let
     [records
      (if header (rest rows) rows)
      body
      (mapcat
       (fn
        [row]
        (if
         header
         (mapping-events (get+ rows 0) row)
         (collection-events "sequence" (map scalar-event row))))
       records)
      prefix
      [(event "stream_start") (event "document_start")]
      suffix
      [(event "document_end") (event "stream_end")]]
     (vec (concat prefix (collection-events "sequence" body) suffix)))))))
(defn
 parse-options
 [config environment]
 (when-not (map? config) (fail "configuration must be a mapping"))
 (let
  [unknown
   (remove (fn [& [_1]] (in? _1 [:format :header])) (keys config))]
  (when
   (seq unknown)
   (fail (str "unknown configuration keys " (pr-str unknown))))
  (let
   [environment
    (if (nil? environment) "auto" environment)
    format
    (if
     (+contains? config :format)
     (get+ config (quote format))
     environment)]
   (when-not
    (in? format (qw csv tsv auto))
    (fail "format must be csv, tsv, or auto"))
   (let
    [header
     (if (+contains? config :header) (get+ config (quote header)) true)]
    (when-not (boolean? header) (fail "header must be a boolean"))
    (% :format format :header header)))))
(defn-
 detect-format
 [text]
 (loop
  [chars (seq text) quoted false started false found #{}]
  (if
   (empty? chars)
   (select-format found)
   (do
    (let
     [c (first chars)]
     (cond
      (= c (C 34))
      (recur (rest chars) (not quoted) true found)
      quoted
      (recur (rest chars) quoted true found)
      (= c (C 10))
      (if
       started
       (select-format found)
       (recur (rest chars) false false found))
      (in? c [(C 44) (C 9)])
      (recur (rest chars) false true (conj found c))
      :else
      (recur (rest chars) false (or started (not= c (C 13))) found)))))))
(defn-
 select-format
 [found]
 (when
  (gt (count found) 1)
  (fail "ambiguous delimiter; set format to csv or tsv"))
 (if (has? found (C 9)) "tsv" "csv"))
(defn-
 validate-header
 [header]
 (when (has? header "") (fail "header names must not be empty"))
 (when
  (not= (count header) (count (distinct header)))
  (fail "header names must be unique")))
(defn-
 mapping-events
 [header row]
 (let
  [fields (+map (interleave header row) scalar-event)]
  (collection-events "mapping" fields)))
(defn-
 collection-events
 [kind items]
 (concat
  [(put (event (str kind "_start")) :flow false)]
  items
  [(event (str kind "_end"))]))
(defn-
 scalar-event
 [value]
 (% :event "scalar" :tag "tag:yaml.org,2002:str" :value value))
(defn- event [kind] (% :event kind))
(defn- fail [message] (throw (ex-info message (%))))
