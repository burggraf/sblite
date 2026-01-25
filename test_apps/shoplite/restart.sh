cd ../.. && go build . && cp sblite test_apps/shoplite \
&& cd test_apps/shoplite \
&& pnpm run build \
&& ../../sblite serve \
	--db shoplite.db \
	--functions \
	--static-dir dist \
	--pg-port 5432
