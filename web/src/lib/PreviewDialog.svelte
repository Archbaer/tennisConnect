<script lang="ts">
  import { tick } from 'svelte';
  import { weekdayLabel, type Profile } from '#lib/api';

  export let profile: Profile;
  export let photoUrl = '';
  export let area = '';

  let dialog: HTMLDialogElement;
  let heading: HTMLHeadingElement;

  $: initials = profile.display_name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('');

  $: days = new Set(profile.availability.map((slot) => slot.weekday));
  $: playSummary = profile.availability.length
    ? `${profile.availability
        .slice(0, 2)
        .map((slot) => weekdayLabel(slot.weekday))
        .join(', ')}${profile.availability.length > 2 ? '…' : ''}`
    : 'No play times yet';

  export async function open() {
    if (!dialog.open) dialog.showModal();
    await tick();
    heading?.focus();
  }

  function close() {
    dialog.close();
  }
</script>

<dialog bind:this={dialog} class="preview-dialog" aria-labelledby="preview-title">
  <div class="dialog-head">
    <div>
      <p class="score mb-1">Preview</p>
      <h2 id="preview-title" class="mb-0" tabindex="-1" bind:this={heading}>Your card</h2>
    </div>
    <button class="icon-button dark" type="button" onclick={close} aria-label="Close preview">
      <i class="bi bi-x-lg" aria-hidden="true"></i>
    </button>
  </div>

  <article class="preview-card">
    <div class="preview-photo">
      <div class="preview-fallback" aria-hidden="true">{initials}</div>
      {#if photoUrl}
        <img src={photoUrl} alt={`${profile.display_name}'s photo`} onerror={(event) => ((event.currentTarget as HTMLImageElement).hidden = true)} />
      {/if}
      {#if area}<span class="chip preview-chip">{area}</span>{/if}
    </div>
    <div class="preview-body">
      <div class="name-row">
        <h3 class="mb-0">{profile.display_name}{profile.age ? `, ${profile.age}` : ''}</h3>
        <span class="chip">{profile.skill_level}</span>
      </div>
      <div class="week-strip" aria-hidden="true">
        {#each [1, 2, 3, 4, 5, 6, 7] as day}
          <span class:active={days.has(day)}>{weekdayLabel(day)[0]}</span>
        {/each}
      </div>
      <dl class="meta">
        <div><dt class="score">Plays</dt><dd>{playSummary}</dd></div>
        <div><dt class="score">Area</dt><dd>{area || 'Not set'}</dd></div>
      </dl>
      {#if profile.bio}<p class="muted mb-0">{profile.bio}</p>{/if}
      <div class="preview-actions" aria-hidden="true">
        <span class="btn btn-primary">I'd play</span>
        <span class="btn btn-outline-secondary">Not for me</span>
      </div>
    </div>
  </article>
  <p class="muted small mb-0">This is read-only. Other players see your approved photo, first name, age, level, play summary and coarse area.</p>
</dialog>

<style>
  .preview-dialog {
    width: min(100% - 2rem, 30rem);
    max-height: 92dvh;
    margin: auto;
    padding: 1.25rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-top), var(--surface) 72%);
    color: var(--ink);
    box-shadow: var(--card-highlight), var(--tc-shadow-3);
    overflow-y: auto;
  }

  .preview-dialog::backdrop {
    background: var(--backdrop);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
  }

  .dialog-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 0.75rem;
  }

  .dialog-head h2:focus {
    outline: none;
  }

  .preview-card {
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: linear-gradient(180deg, var(--surface-top), var(--surface) 72%);
    box-shadow: var(--card-highlight), var(--tc-shadow-1);
  }

  .preview-photo {
    position: relative;
    width: 100%;
    aspect-ratio: 4 / 3;
    overflow: hidden;
    background: var(--media-bg);
  }

  .preview-photo img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .preview-fallback {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background:
      linear-gradient(135deg, rgb(216 255 62 / 18%), transparent 55%),
      linear-gradient(160deg, var(--court-light), var(--court-deep));
    color: var(--ball);
    font-size: 4rem;
    font-weight: 800;
  }

  .preview-chip {
    position: absolute;
    left: 0.75rem;
    top: 0.75rem;
    background: rgb(255 255 255 / 92%);
  }

  .preview-body {
    display: grid;
    gap: 0.6rem;
    padding: 1rem;
  }

  .name-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .week-strip {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 0.25rem;
  }

  .week-strip span {
    display: grid;
    place-items: center;
    min-height: 1.7rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-chip);
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 700;
  }

  .week-strip span.active {
    border-color: var(--court);
    background: var(--court);
    color: white;
  }

  .meta {
    display: grid;
    gap: 0.35rem;
    margin: 0;
    padding-block: 0.45rem;
    border-block: 1px solid var(--line);
  }

  .meta > div {
    display: flex;
    justify-content: space-between;
    gap: 1rem;
  }

  .meta dt {
    color: var(--muted);
  }

  .meta dd {
    margin: 0;
    text-align: right;
    font-weight: 700;
  }

  .preview-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.5rem;
    padding-top: 0.25rem;
  }

  .preview-actions .btn {
    min-height: 2.8rem;
    pointer-events: none;
  }

  .small {
    margin-top: 0.75rem;
    font-size: 0.82rem;
  }
</style>
