<script lang="ts">
  import { apiJSON } from '#lib/api';
  import { online } from '#lib/offline';

  export let initialArea = '';
  export let onSaved: (label: string) => void;

  let area = '';
  let areaLabel = initialArea;
  let areaError = '';
  let permissionError = '';
  let locating = false;
  let saving = false;

  function useLocation() {
    permissionError = '';
    if (!navigator.geolocation) {
      permissionError = 'This browser does not provide a location. Enter a postcode or area instead.';
      return;
    }
    locating = true;
    navigator.geolocation.getCurrentPosition(
      async (position) => {
        try {
          const result = await apiJSON<{ area_label: string }>('/api/v1/location', {
            method: 'PUT',
            body: JSON.stringify({
              latitude: position.coords.latitude,
              longitude: position.coords.longitude
            })
          });
          areaLabel = result.area_label;
          onSaved(result.area_label);
        } catch (problem) {
          permissionError =
            problem instanceof Error ? problem.message : 'We could not use that location.';
        } finally {
          locating = false;
        }
      },
      (problem) => {
        locating = false;
        if (problem.code === 1) {
          permissionError =
            'Location permission was denied. Enter a postcode or neighbourhood instead.';
          return;
        }
        permissionError = 'We could not get your location. Enter a postcode or area instead.';
      },
      { timeout: 10000, maximumAge: 0 }
    );
  }

  async function saveArea(event: SubmitEvent) {
    event.preventDefault();
    areaError = '';
    saving = true;
    try {
      const result = await apiJSON<{ area_label: string }>('/api/v1/location', {
        method: 'PUT',
        body: JSON.stringify({ area })
      });
      areaLabel = result.area_label;
      onSaved(result.area_label);
    } catch (problem) {
      areaError = problem instanceof Error ? problem.message : 'We could not find that area.';
    } finally {
      saving = false;
    }
  }
</script>

<div class="stack">
  <p class="muted mb-0">A postcode or neighbourhood. We store a coarse area and never return coordinates.</p>

  {#if areaLabel}
    <p class="notice mb-0">Your area: {areaLabel}</p>
  {/if}
  {#if areaError}
    <p class="alert-box mb-0" role="alert">{areaError}</p>
  {/if}
  {#if permissionError}
    <p class="alert-box mb-0" role="alert">{permissionError}</p>
  {/if}

  <button type="button" class="btn btn-outline-secondary" onclick={useLocation} disabled={locating || !$online}>
    {locating ? 'Finding your area…' : 'Use my location'}
  </button>
  <small class="muted">Browser location is sent to Mapbox and stored as a coarse area.</small>

  <form class="stack" onsubmit={saveArea}>
    <label>
      Postcode or area
      <input class="form-control" required bind:value={area} placeholder="SW11 4NJ or Battersea" />
    </label>
    <div class="sticky-actions save-bar">
      <button class="btn btn-primary" type="submit" disabled={saving || !$online}>
        {saving ? 'Saving…' : 'Save area'}
      </button>
    </div>
  </form>
</div>
