import type { ProviderInfo, Provider } from '../types';

interface ModelSelectorProps {
  providers: ProviderInfo[];
  selectedProvider: Provider | null;
  selectedModel: string | null;
  onSelect: (provider: Provider, model: string) => void;
  disabled?: boolean;
}

export function ModelSelector({
  providers,
  selectedProvider,
  selectedModel,
  onSelect,
  disabled = false,
}: ModelSelectorProps) {
  const availableProviders = providers.filter((p) => p.available);

  if (availableProviders.length === 0) {
    return (
      <div className="model-selector disabled">
        <span className="no-models">No LLM providers configured</span>
      </div>
    );
  }

  const currentValue =
    selectedProvider && selectedModel
      ? `${selectedProvider}:${selectedModel}`
      : '';

  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const [provider, model] = e.target.value.split(':') as [Provider, string];
    onSelect(provider, model);
  };

  return (
    <div className="model-selector">
      <label htmlFor="model-select">Model:</label>
      <select
        id="model-select"
        value={currentValue}
        onChange={handleChange}
        disabled={disabled}
        title="Select the LLM provider and model for compilation"
      >
        {availableProviders.map((provider) => (
          <optgroup key={provider.id} label={provider.name}>
            {provider.models.map((model) => (
              <option key={model.id} value={`${provider.id}:${model.id}`}>
                {model.name}
              </option>
            ))}
          </optgroup>
        ))}
      </select>
    </div>
  );
}
