package all

import (
	_ "github.com/karimra/gnmic/processors/event_convert"
	_ "github.com/karimra/gnmic/processors/event_date_string"
	_ "github.com/karimra/gnmic/processors/event_delete"
	_ "github.com/karimra/gnmic/processors/event_drop"
	_ "github.com/karimra/gnmic/processors/event_print"
	_ "github.com/karimra/gnmic/processors/event_replace"
	_ "github.com/karimra/gnmic/processors/event_to_tag"
)
