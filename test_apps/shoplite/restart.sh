cd ../../sblite && go build . && cp sblite test_apps/shoplite \
&& cd test_apps/shoplite \
&& ./sblite serve --db shoplite.db

