<script lang="ts">
  import { beforeNavigate } from '$app/navigation';
  import { onMount } from 'svelte';
  import { apiJSON, weekdayLabel, type Availability, type Profile } from '#lib/api';
  import { online } from '#lib/offline';

  export let initial: Profile | null = null;
  export let submitLabel = 'Save profile';
  export let showPreferences = true;
  export let showAvailability = true;
  export let onSaved: (() => void) | undefined = undefined;

  const defaults: Availability[] = [
    { weekday: 2, start: '18:00', end: '21:00' },
    { weekday: 4, start: '18:00', end: '21:00' },
    { weekday: 7, start: '10:00', end: '13:00' }
  ];

  let form: Profile = {
    display_name: initial?.display_name ?? '',
    gender: initial?.gender ?? 'woman',
    skill_level: initial?.skill_level ?? 'intermediate',
    ranking: initial?.ranking ?? '',
    bio: initial?.bio ?? '',
    gender_preference: initial?.gender_preference ?? 'all',
    min_age: initial?.min_age ?? 18,
    max_age: initial?.max_age ?? 120,
    skill_tolerance: initial?.skill_tolerance ?? 1,
    availability: initial?.availability?.length ? initial.availability : defaults,
    instagram_handle: initial?.instagram_handle ?? ''
  };
  let error = '';
  let saving = false;
  let snapshot = JSON.stringify(form);
  let dirty = false;

  $: dirty = JSON.stringify(form) !== snapshot;

  beforeNavigate((navigation) => {
    if (!dirty) return;
    if (!window.confirm('Discard your changes?')) navigation.cancel();
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
    form.availability = [...form.availability, { weekday: 6, start: '10:00', end: '12:00' }];
  }

  function removeSlot(index: number) {
    form.availability = form.availability.filter((_, position) => position !== index);
  }

  async function save(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    saving = true;
    try {
      await apiJSON('/api/v1/profile', {
        method: 'PUT',
        body: JSON.stringify({
          ...form,
          ranking: form.ranking || null,
          instagram_handle: form.instagram_handle || null,
          availability: form.availability.map((slot) => ({
            weekday: Number(slot.weekday),
            start: slot.start,
            end: slot.end
          })),
          min_age: Number(form.min_age),
          max_age: Number(form.max_age),
          skill_tolerance: Number(form.skill_tolerance)
        })
      });
      snapshot = JSON.stringify(form);
      dirty = false;
      onSaved?.();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not save that profile.';
    } finally {
      saving = false;
    }
  }
</script>

<form class="stack" onsubmit={save}>
  {#if error}
    <p class="alert-box" role="alert">{error}</p>
  {/if}

  <div class="grid two">
    <label>
      Display name
      <input class="form-control" name="display_name" required maxlength="50" bind:value={form.display_name} />
    </label>
    <label>
      Level
      <select class="form-select" bind:value={form.skill_level}>
        <option value="beginner">Beginner</option>
        <option value="improver">Improver</option>
        <option value="intermediate">Intermediate</option>
        <option value="advanced">Advanced</option>
      </select>
    </label>
  </div>

  <div class="grid two">
    <label>
      You are
      <select class="form-select" bind:value={form.gender}>
        <option value="woman">Woman</option>
        <option value="man">Man</option>
      </select>
    </label>
    {#if showPreferences}
      <label>
        Show me
        <select class="form-select" bind:value={form.gender_preference}>
          <option value="all">Everyone</option>
          <option value="women">Women</option>
          <option value="men">Men</option>
        </select>
      </label>
    {/if}
  </div>

  <label>
    Short bio
    <textarea class="form-control" maxlength="200" bind:value={form.bio}></textarea>
    <small>{form.bio.length}/200. No phone numbers, e-mail addresses or dates written as numbers.</small>
  </label>

  <label>
    Ranking (optional, self-reported)
    <input class="form-control" maxlength="50" bind:value={form.ranking} />
  </label>

  <label>
    Instagram (optional, shared only after a mutual match)
    <input class="form-control" bind:value={form.instagram_handle} placeholder="yourhandle" autocomplete="off" />
  </label>

  {#if showPreferences}
    <fieldset>
      <legend>Age range you would like to meet</legend>
      <div class="grid two">
        <label>
          Minimum
          <input class="form-control" type="number" min="18" max="120" bind:value={form.min_age} />
        </label>
        <label>
          Maximum
          <input class="form-control" type="number" min="18" max="120" bind:value={form.max_age} />
        </label>
      </div>
    </fieldset>

    <label>
      Skill tolerance
      <select class="form-select" bind:value={form.skill_tolerance}>
        <option value={0}>Exact level only</option>
        <option value={1}>Include the adjacent level</option>
      </select>
    </label>
  {/if}

  {#if showAvailability}
    <fieldset>
      <legend>When you play</legend>
      <p class="muted">Europe/London time. Slots must stay inside one day.</p>
      <div class="stack">
        {#each form.availability as slot, index (index)}
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
              disabled={form.availability.length === 1}
              aria-label={`Remove ${weekdayLabel(slot.weekday)} slot`}
            >
              Remove
            </button>
          </div>
        {/each}
      </div>
      <button type="button" class="btn btn-outline-secondary" onclick={addSlot} disabled={form.availability.length >= 20}>
        Add a slot
      </button>
      {#if form.availability.length === 1}
        <small class="muted">You need at least one play time. This one cannot be removed.</small>
      {/if}
    </fieldset>
  {/if}

  <button class="btn btn-primary" type="submit" disabled={saving || !$online}>{saving ? 'Saving…' : submitLabel}</button>
</form>

<style>
  fieldset {
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 1rem;
  }

  legend {
    font-weight: 700;
    padding-inline: 0.35rem;
  }

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
