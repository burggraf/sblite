pnpm run build
../../sblite serve \
    --db shoplite.db \
    --functions \
    --static-dir dist \
    --pg-port 5432

