<script lang="ts">
  import { beforeNavigate } from '$app/navigation';
  import { onMount } from 'svelte';
  import { apiJSON, weekdayLabel, type Availability, type Profile } from '#lib/api';
  import { online } from '#lib/offline';
  import { showToast } from '#lib/toasts';

  export let profile: Profile;
  export let onSaved: (() => void) | undefined = undefined;

  let slots: Availability[] = profile.availability.map((slot) => ({ ...slot }));
  let snapshot = JSON.stringify(slots);
  let dirty = false;
  let saving = false;
  let error = '';

  $: dirty = JSON.stringify(slots) !== snapshot;

  beforeNavigate((navigation) => {
    if (!dirty) return;
    if (!window.confirm('Discard your unsaved play times?')) navigation.cancel();
  });

  onMount(() => {
    const warn = (event: BeforeUnloadEvent) => {
      if (!dirty) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  });

  function addSlot() {
    slots = [...slots, { weekday: 6, start: '10:00', end: '12:00' }];
  }

  function removeSlot(index: number) {
    if (slots.length === 1) return;
    slots = slots.filter((_, position) => position !== index);
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    saving = true;
    try {
      await apiJSON('/api/v1/profile', {
        method: 'PUT',
        body: JSON.stringify({
          ...profile,
          availability: slots.map((slot) => ({
            weekday: Number(slot.weekday),
            start: slot.start,
            end: slot.end
          }))
        })
      });
      snapshot = JSON.stringify(slots);
      dirty = false;
      showToast('Play times saved.');
      onSaved?.();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not save your play times.';
    } finally {
      saving = false;
    }
  }
</script>

<form class="stack" onsubmit={save}>
  <p class="notice mb-0">Europe/London time. Slots must stay inside one day.</p>
  <div class="stack">
    {#each slots as slot, index (index)}
      <div class="slot-row">
        <label>
          Day
          <select class="form-select" bind:value={slot.weekday}>
            {#each [1, 2, 3, 4, 5, 6, 7] as day}
              <option value={day}>{weekdayLabel(day)}</option>
            {/each}
          </select>
        </label>
        <label>
          From
          <input class="form-control" type="time" bind:value={slot.start} />
        </label>
        <label>
          To
          <input class="form-control" type="time" bind:value={slot.end} />
        </label>
        <button
          type="button"
          class="btn btn-outline-secondary"
          onclick={() => removeSlot(index)}
          disabled={slots.length === 1}
          aria-label={`Remove ${weekdayLabel(slot.weekday)} slot`}
        >
          Remove
        </button>
      </div>
    {/each}
  </div>
  <button type="button" class="btn btn-outline-secondary align-self-start" onclick={addSlot} disabled={slots.length >= 20}>
    Add a slot
  </button>
  {#if slots.length === 1}
    <small class="muted">You need at least one play time. This one cannot be removed.</small>
  {/if}
  {#if error}<p class="alert-box mb-0" role="alert">{error}</p>{/if}
  <div class="sticky-actions save-bar">
    <button class="btn btn-primary" type="submit" disabled={saving || !$online}>
      {saving ? 'Saving…' : 'Save play times'}
    </button>
  </div>
</form>

<style>
  .slot-row {
    display: grid;
    grid-template-columns: 1fr 1fr 1fr auto;
    gap: 0.5rem;
    align-items: end;
  }

  @media (max-width: 620px) {
    .slot-row {
      grid-template-columns: 1fr 1fr;
    }
  }
</style>
