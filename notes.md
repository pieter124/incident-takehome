# Initial Thoughts

## Given Information
`schedule` is a JSON file containing a definition of a schedule.

`schedule` has the `users` ordered in the order of shift rotation.

`handover_start_at` is the UTC given timestamp of when the schedule rotation begins.

`handover_interval_days` is the number of days until the schedule shifts to the next user.

`overrides` is a JSON file containing an array of overrides.

Overrides will always be after the schedule's `handover_start_at`.

Overrides will not overlap with each other.

Output should be a JSON array of the final schedule as a list of entries.

Final schedule should be truncated based on the provided `from` and `until`.

## Assumptions 
Assume any time in given as a string in UTC format.
Assume `handover_interval_days` is a positive integer > 0.
Assume that overrides may contain users that aren't defined in schedule.
Don't assume `from` >= `handover_start_at`, not actually guaranteed by the brief. Make this instead: `effective_from = max(from, handover_start_at)`, since nobody is on-call before the schedule starts.
Assume that there is always someone on-call (so no gaps and the `end_at` of one shift is the `start_at` of another shift).
Overrides CAN cover the same user's own shift. This contradicts what I originally wrote here. Step 7 (merging consecutive same-user entries) already handles this fine, so no special-casing needed, just don't skip applying an override because the user matches.

## Blurb out thoughts
Reject `from` > `until` (from after until doesn't make sense) - had the direction backwards originally.
If `from` == `until`, that's a zero-length window. Given the invariant below (`start_at < end_at` for every entry), a zero-length window can't contain a valid entry. Decided to reject this the same way as `from` > `until`, one rule, one error path, instead of giving it its own empty array branch. "Who is on-call right now" is a genuinely different question worth answering some other way - see product features below - not by overloading this flag.
If no schedule file is provided, output appropriate message.
If no schedule file exists, output appropriate message (either `FILE_NAME.json` doesn't exist or "You did not provide a valid path to a `schedule.json` file").
If valid schedule file was provided, but overrides file wasn't provided, we can still render our schedule.
If `--overrides` was provided but the file isn't valid/parseable, fail loudly (error to stderr, non-zero exit) rather than silently rendering without it. Silently dropping overrides and rendering the un-overridden schedule means paging the wrong person, which is worse than just refusing to run. (Not providing `--overrides` at all is still fine, that's just "no overrides".)
Do basic parsing and validation for body of `schedule.json` and `overrides.json`.

We care about correctness as a priority, testing is a good idea?
Is UTC lexicographically sortable? Only if every timestamp string is formatted identically (same precision, same `Z` vs offset notation) - not safe to rely on in general, since e.g. `...17:00:00Z` and `...17:00:00.000Z` represent the same instant but sort differently as strings. Safer to parse everything to an actual time value up front and only use strings for output.

## How to solve in steps
1. Validate input (`from` > `until` is invalid, schedule file exists and is a valid JSON object following the schema above, overrides file if provided is a valid JSON array following the schema above)
    After this point, all input should be valid for a correct and valid schedule to be outputted.
2. Using schedule, generate the base entries, starting from `handover_start_at` and stop generating when the `start_at` of the next-to-be generated entry is >= `until` (not `>` - if the next `start_at` lands exactly on `until` it has zero overlap with the window, and letting it through would produce a zero-length entry after truncation, breaking the invariant below).
    So at this point, the most recently generated entry (and generated in chronological order), will have `end_at` >= `until`. There can also be entries at the start of the generated list with `end_at` <= `from`.
3. Apply overrides to this full base entry list, before any removing or truncating happens.
    Need to discuss finer low level implementation details. One case to handle is an override that spans across a handover boundary, so it needs to be split against more than one base entry, even if the same user is overriding themselves.
    Sorting the overrides chronologically happens inside this step, not as a separate step beforehand. Moved this in from where it used to be (there is no separate step for it any more), so the function doesn't rely on the caller having already sorted its input.
4. Remove entries with `end_at` <= `from`.
    At this point, there should be no more removals required from our generated list.
5. Truncate first entry using `max(start_at, from)`.
    At this point, all entries should have times >= `from`.
6. Truncate last entry using `min(end_at, until)`.
   At this point, for all entries e: `from` <= `e.start_at` < `e.end_at` <= `until`. This step does not need to know whether an entry came from the base rotation or from an override, it is only clipping a list of entries to the window.
7. Merge any consecutive entries with the same user (assuming due to override carrying over or ending as the same user's shift started to name a few examples.)
8. Output to stdout.

Decided on the ordering question. Apply overrides right after step 2, before removing or truncating anything, instead of applying them last. Doing it in this order means the override splicing code only ever has to deal with the base rotation and never needs to know about `from` or `until`. Truncation is just keeping the part of the timeline inside the window, and that does not depend on whether an entry came from the base rotation or from an override, so doing overrides before truncation gives the same final answer as doing them after. It just keeps each piece of logic smaller and easier to test on its own.

## Good practices from my scheduler.go draft

Wrote this about a month ago as prep. The step order and some of the reasoning is different from what I just decided above, but a few implementation choices held up well and are worth carrying over.

- Works out which base shift a given time falls into directly with integer division and a floor mod for the user index, instead of looping forward from handover_start_at. This is the epoch jump I was worried about in the assumptions section. Decided not to carry this one over. Even the epoch case is only a loop of a few tens of thousands of iterations, which is still instant, so there is no real performance reason to avoid the simple loop. The loop is easier to read and easier to trust is correct, so step 2 above just loops forward from handover_start_at and that is the version to build.
- Parses every timestamp into a real time.Time as soon as it is read, so nothing downstream ever compares timestamp strings. This avoids the lexicographic sorting trap noted above.
- Truncate is one small generic function that clips any list of entries to a window, and it gets reused both for the generated base entries and for the overrides read from file. Same logic, no duplication.
- Overlap checking for overrides is a single sort plus one pass comparing each entry's start against the previous entry's end.
- The override splicing function writes its precondition directly in a comment, that overrides must already be non overlapping, and explains why that makes a single pass over them safe.
- File reading and file writing are small separate helpers, not tied to the schedule or override shape, so the same functions get reused for both input files and the optional output file.
- Errors use log.Fatal, which prints to stderr and exits non zero. Matches the fail loudly decision made above for a bad overrides file.
- A one line log summary of user count, override count and window is printed before the JSON, on stderr through log, so stdout stays clean JSON that can be piped elsewhere.

## TDD plan

Write tests before the code, in this order, roughly matching the pipeline above. Each stage is its own small pure function so it can be tested on its own before wiring everything together.

1. Validation. No dependency on anything else so start here. Cases to cover: from after until, schedule file missing, schedule file not valid json, schedule missing a required field, interval not a positive integer, overrides file not provided at all (should be fine), overrides file provided but not valid json or not following the shape (should fail loudly with a non zero exit).
2. Base entry generation. Test the function that takes the schedule and a window and returns the untruncated rotation entries. Cases to cover: one user, several users, a window that starts partway through the rotation (the epoch example), a window that ends exactly on a handover boundary, interval of one day and interval of several days.
3. Applying overrides. Test the function that takes the base entries and the sorted overrides and returns a new list with overrides spliced in. Cases to cover: override fully inside one base entry, override that exactly matches one base entry, override that spans across a handover so it touches two base entries, override right at the start or end of the generated range, a user overriding their own shift, several overrides in a row.
4. Removing and truncating. Test this as a plain clipping function, independent of where the entries came from. Give it any list of entries and a window and check it drops anything fully outside the window and clips the first and last entries to the window. This is also where from equal to until and from before handover start at get tested.
5. Merging. Test that consecutive entries with the same user get combined into one after overrides and truncation. Cases to cover: an override that starts and ends with the same surrounding user so it disappears completely, an override that lines up exactly with a handover.
6. End to end. Wire validation, generation, overrides, clipping and merging together into one BuildSchedule function. Use the README example as the main end to end test, no need for a separate case per edge case here since every step already has its own tests above.

## What other product features on top of this?

- Display schedule in terms of local time for users.
- Multi-tier escalation (primary/secondary/manager) instead of a single rotation. If primary doesn't ack within N minutes, escalate to the next layer.
- Coverage gap / unstaffed shift detection - warn if a schedule has nobody assigned for some period (e.g. someone is removed from `users` but still has an override queued, or a rotation ends with no successor configured).
- Fairness/load metrics - nights, weekends, and total hours covered per user over a period, to collect information and catch rotas which are uneven.
- Override/swap requests with approval flow - one person requests a swap, the other accepts, then it becomes an override automatically. Needs an audit trail (who changed what, when, why).
- Calendar export per user or per schedule, so shifts show up in people's normal calendar apps.
- Notifications ahead of a shift starting ("you're on-call in 24h"), and reminders to the next person if a handover is approaching with no coverage configured.

## How to productionise this

- Wrap the core logic (the pure function: schedule + overrides + window -> entries) behind an API endpoint, and keep this CLI as a thin wrapper calling the same function, so the logic only exists once.
- Schedules and overrides change over time (people join/leave, overrides get added later), so these need their own history/versioning in storage, not just "latest JSON" - want to be able to answer "what did the schedule look like as of last Tuesday" for audits/incident reviews.
- Precompute and cache rendered entries for the near future (e.g. next 2 weeks), invalidating the cache when a schedule or override changes, rather than recomputing on every page/notification send - this render is on the hot path that actually pages someone, so it needs to be fast and reliable.
- Concurrent edits: two people adding overrides to the same schedule at the same time needs real handling (optimistic concurrency / locking), given overrides must not overlap with each other.
- Timezones: store everything in UTC internally (as I'm already doing), and only convert to a viewer's local timezone at render/display time - never store local times.
- Observability: log/alert on render errors or on "nobody is on-call right now" results, since that's equivalent to a page silently not going out - should be treated as a critical path failure, not a normal error.
- Formal verification (TLA+): model the layered rotation/override logic as a spec and use TLC to check the "always exactly one on-call user, no gaps or overlaps" invariant across every boundary-time case, rather than relying only on hand-picked unit tests. Overkill for the take-home, but the kind of guarantee incident.io would want for the real product.
