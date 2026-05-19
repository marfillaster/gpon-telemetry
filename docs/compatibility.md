# Firmware Compatibility Notes

The parser is deliberately small. It is designed for pages that look like the
Realtek SDK `status_pon.asp` page:

```html
<td><font size=2><b>Temperature</b></td>
<td><font size=2>54.898438 C</td>
```

The scraper finds a `<b>Label</b>` cell, then reads the text in the next value
cell. It does not need JavaScript and does not depend on exact table widths,
colors, or row ordering.

## Expected Login

```sh
curl 'http://192.168.1.1/boaform/admin/formLogin' \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  --data-raw 'challenge=&username=admin&password=admin&save=Login&submit-url=%2Fadmin%2Flogin.asp'
```

Some firmware returns an "already logged in" HTML page when a session already
exists. That is acceptable as long as `GET /status_pon.asp` succeeds
afterward.

## Expected Status Labels

- `Temperature`
- `Voltage`
- `Tx Power`
- `Rx Power`
- `Bias Current`
- `ONU State`

If a sibling firmware uses different labels, the clean extension point is to
make label names configurable by environment variable rather than adding a new
polling transport.
