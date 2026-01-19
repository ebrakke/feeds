# Quick Feed Creation from Video Card Menu

## Overview

Add an "Add to feed" submenu to the video card dropdown menu, allowing users to add a video's channel to any existing feed or create a new feed on the fly while browsing.

## User Flow

1. User hovers/taps video card menu (three dots)
2. Clicks "Add to feed..."
3. Submenu appears showing:
   - List of existing feeds (excluding current feed and feeds that already contain this channel)
   - Divider line
   - "Create new feed..." option at bottom
4. **If selecting existing feed:** Channel is added, toast confirms "Added to [Feed Name]"
5. **If selecting "Create new feed...":** Browser prompt asks for feed name → feed is created → channel is added → toast confirms "Added to [New Feed Name]"

## Technical Implementation

### Backend

No changes needed. Existing APIs support the full flow:

- `GET /api/feeds` - fetch list of feeds for the submenu
- `POST /api/feeds` - create new feed (body: `{name}`)
- `POST /api/channels/{id}/feeds` - add channel to feed (body: `{feedId}`)
- `GET /api/channels/{id}` - get channel's current feed memberships

### Frontend Changes

#### 1. VideoCard.svelte

Add "Add to feed..." menu item with nested submenu:

- Fetch feeds list when submenu opens
- Fetch channel details to know which feeds it's already in
- Filter out: current feed + feeds already containing the channel
- Show "Create new feed..." option at bottom with divider
- Handle create flow: `prompt()` → `createFeed()` → `addChannelToFeed()`
- Show toast notification on success/failure

#### 2. Toast System

Add a simple toast notification system if one doesn't exist:

- Auto-dismiss after 3 seconds
- Support success/error variants
- Stack multiple toasts if needed

### Data Flow

```
User clicks "Add to feed..."
        │
        ▼
Fetch feeds list (GET /api/feeds)
Fetch channel feeds (GET /api/channels/{id})
        │
        ▼
Display submenu with filtered feeds
        │
        ├─► User selects existing feed
        │           │
        │           ▼
        │   POST /api/channels/{id}/feeds
        │           │
        │           ▼
        │   Show toast "Added to [Feed Name]"
        │
        └─► User selects "Create new feed..."
                    │
                    ▼
            Browser prompt() for name
                    │
                    ▼
            POST /api/feeds (create feed)
                    │
                    ▼
            POST /api/channels/{id}/feeds
                    │
                    ▼
            Show toast "Added to [New Feed Name]"
```

### Component Props

VideoCard currently receives:
- `video` - includes `channel_id` and `channel_name`
- `feedId` - current feed being viewed (optional)
- `showRemoveFromFeed` - whether to show remove option

No new props needed - component fetches what it needs when submenu opens.

## UI Details

### Submenu Appearance

```
┌─────────────────────────┐
│ Watch on YouTube        │
│ Mark as watched         │
│ Add to feed...        ► │──┬─────────────────────┐
│ Remove from feed        │  │ Gaming              │
└─────────────────────────┘  │ Music               │
                             │ Tech Reviews        │
                             │─────────────────────│
                             │ Create new feed...  │
                             └─────────────────────┘
```

### Toast Appearance

```
┌─────────────────────────────────┐
│ ✓ Added to Gaming               │
└─────────────────────────────────┘
```

## Edge Cases

1. **No other feeds exist:** Submenu shows only "Create new feed..."
2. **Channel already in all feeds:** Submenu shows only "Create new feed..."
3. **Create feed cancelled:** User clicks Cancel on prompt - no action taken
4. **Empty feed name:** Validate and show error, re-prompt or cancel
5. **API error:** Show error toast, submenu closes

## Files to Modify

1. `/web/frontend/src/lib/components/VideoCard.svelte` - Add submenu logic
2. `/web/frontend/src/lib/components/Toast.svelte` - New component (if needed)
3. `/web/frontend/src/lib/stores/toast.ts` - Toast state management (if needed)
4. `/web/frontend/src/routes/+layout.svelte` - Mount toast container (if needed)
