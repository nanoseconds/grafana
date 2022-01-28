import React, { useMemo, useCallback } from 'react';
import { mapValues } from 'lodash';
import { toPairs } from 'lodash';
import { values } from 'lodash';
import { head } from 'lodash';
import { keys } from 'lodash';

import {
  ReducerID,
  FieldType,
  SelectableValue,
  DataTransformerID,
  TransformerUIProps,
  getFieldDisplayName,
  standardTransformers,
  TransformerRegistyItem,
  DataFrame,
} from '@grafana/data';
import { StatsPicker, FilterPill, HorizontalGroup, Button, Select } from '@grafana/ui';
import {
  CalculateToRowOptions,
  RowPlacement,
  rowPlacements,
  CalculateMode,
  calculateModes,
} from '@grafana/data/src/transformations/transformers/calculateToRow';
import { useAllFieldNamesFromDataFrames } from './utils';

const DEFAULT_REDUCERS = [ReducerID.sum];

const BasicConfigRow: React.FC<TransformerUIProps<CalculateToRowOptions>> = ({ input, options, onChange }) => {
  const placement = options.placement || RowPlacement.bottom;
  const mode = options.mode || CalculateMode.separated;

  const fieldNames = useAllFieldNamesFromDataFrames(input);
  const fieldTypes = useMemo(() => getFieldTypesFromDataFrame(input), [input]);
  const fieldNameOptions = fieldNames.map((name: string) => ({ label: name, value: name, type: fieldTypes[name] }));

  const onFieldChange = useCallback(
    (key: string, selectable?: SelectableValue<RowPlacement | CalculateMode>) => {
      if (!selectable?.value) {
        return;
      }
      let reducers = options.reducers;

      if (key === 'mode') {
        if (selectable.value === CalculateMode.separated) {
          reducers = {};
        } else {
          reducers = fieldNameOptions
            .filter(option => option.type !== FieldType.time)
            .map(option => [option.value, DEFAULT_REDUCERS] as [string, ReducerID[]])
            .reduce(
              (prev, curr) => ({
                ...prev,
                [curr[0]]: curr[1],
              }),
              {}
            );
        }
      }

      onChange({
        ...options,
        reducers,
        [key]: selectable.value,
      });
    },
    [options, onChange, input, fieldNameOptions]
  );

  return (
    <div className="gf-form-inline">
      <div className="gf-form gf-form-spacing">
        <div className="gf-form-label width-7">Mode</div>
        <Select
          className="width-15"
          placeholder="Default to calculate separately"
          options={calculateModes}
          value={mode}
          onChange={selected => onFieldChange('mode', selected)}
        />
      </div>
      <div className="gf-form gf-form-spacing">
        <div className="gf-form-label width-7">Place at</div>
        <Select
          className="width-15"
          placeholder="(Default to bottom)"
          options={rowPlacements}
          value={placement}
          onChange={selected => onFieldChange('placement', selected)}
        />
      </div>
    </div>
  );
};

const SeparatedCalculationEditor: React.FC<TransformerUIProps<CalculateToRowOptions>> = ({
  input,
  options,
  onChange,
}) => {
  const fieldNames = useAllFieldNamesFromDataFrames(input);
  const fieldTypes = useMemo(() => getFieldTypesFromDataFrame(input), [input]);
  const fieldNameOptions = fieldNames.map((name: string) => ({ label: name, value: name, type: fieldTypes[name] }));

  const reducers = options.reducers || {};
  const unselected = fieldNames.filter(name => !(name in reducers));

  const onAddField = useCallback(() => {
    const unselectedOptions = fieldNameOptions.filter(option => !(option.value in reducers));
    let option =
      unselectedOptions.find(option => option.type !== FieldType.time) ||
      unselectedOptions.find(option => option.type === FieldType.time);

    if (!option) {
      return;
    }

    onChange({
      ...options,
      reducers: {
        ...reducers,
        [option.value]: [] as ReducerID[],
      },
    });
  }, [options, onChange, input, reducers, fieldNameOptions]);

  const onChangeField = useCallback(
    (selectable: SelectableValue<string>, prevField: string) => {
      if (!selectable?.value) {
        return;
      }

      const { [prevField]: _, ...next } = reducers;

      onChange({
        ...options,
        reducers: {
          ...next,
          [selectable.value]: reducers[prevField],
        },
      });
    },
    [options, onChange, input, reducers]
  );

  const onDelete = useCallback(
    field => {
      const { [field]: _, ...next } = reducers;

      onChange({
        ...options,
        reducers: { ...next },
      });
    },
    [options, onChange, reducers]
  );

  return (
    <>
      {toPairs(reducers).map(([field, stats], index) => (
        <div key={index} className="gf-form-inline">
          <div className="gf-form gf-form-spacing">
            <div className="gf-form-label width-7">Field</div>
            <Select
              className="min-width-15 max-width-24"
              placeholder="Field Name"
              options={fieldNameOptions.filter(option => option.value === field || !(option.value in reducers))}
              value={field}
              onChange={(selectable: any) => onChangeField(selectable as SelectableValue<string>, field)}
            />
          </div>
          <div className="gf-form gf-form-spacing">
            <div className="gf-form-label width-7">Stat</div>
            <StatsPicker
              className="min-width-15 max-width-24"
              placeholder="Choose Stat"
              stats={stats}
              onChange={vals => {
                onChange({
                  ...options,
                  reducers: {
                    ...reducers,
                    [field]: vals as ReducerID[],
                  },
                });
              }}
            />
          </div>
          <div className="gf-form">
            <Button icon="times" onClick={() => onDelete(field)} variant="secondary" />
          </div>
        </div>
      ))}
      {unselected.length > 0 && (
        <div className="gf-form">
          <Button icon="plus" size="sm" onClick={onAddField} variant="secondary">
            Add feild to calculate
          </Button>
        </div>
      )}
    </>
  );
};

type FieldOption = {
  label: string;
  value: string;
  type: FieldType;
};
const RowCalculationEditor: React.FC<TransformerUIProps<CalculateToRowOptions>> = ({ input, options, onChange }) => {
  const fieldNames = useAllFieldNamesFromDataFrames(input);
  const fieldTypes = useMemo(() => getFieldTypesFromDataFrame(input), [input]);
  const fieldNameOptions: FieldOption[] = fieldNames.map((name: string) => ({
    label: name,
    value: name,
    type: fieldTypes[name],
  }));

  const selected = new Set(keys(options.reducers));
  const selectedReducers = head(values(options.reducers)) || DEFAULT_REDUCERS;

  const onFieldToggle = useCallback(
    (field: FieldOption) => {
      let reducers = {};
      if (selected.size === 1 && selected.has(field.value)) {
        reducers = fieldNameOptions
          .filter(option => option.type !== FieldType.time)
          .map(option => [option.value, selectedReducers] as [string, ReducerID[]])
          .reduce(
            (prev, curr) => ({
              ...prev,
              [curr[0]]: curr[1],
            }),
            {}
          );
      } else {
        const { [field.value]: _, ...rest } = options.reducers;

        if (!selected.has(field.value)) {
          reducers = { [field.value]: selectedReducers };
        }

        reducers = {
          ...reducers,
          ...rest,
        };
      }

      onChange({
        ...options,
        reducers,
      });
    },
    [options, onChange, selected]
  );

  const onStatsChange = useCallback(
    (reducers: ReducerID[]) => {
      onChange({
        ...options,
        reducers: mapValues(options.reducers, () => reducers),
      });
    },
    [options, onChange]
  );

  return (
    <>
      <div className="gf-form-inline">
        <div className="gf-form gf-form--grow">
          <div className="gf-form-label width-7">Field name</div>
          <HorizontalGroup spacing="xs" align="flex-start" wrap>
            {fieldNameOptions.map((field, i) => {
              return (
                <FilterPill
                  key={`${field.value}/${i}`}
                  onClick={() => {
                    onFieldToggle(field);
                  }}
                  label={field.label}
                  selected={selected.has(field.value)}
                />
              );
            })}
          </HorizontalGroup>
        </div>
      </div>
      <div className="gf-form-inline">
        <div className="gf-form">
          <div className="gf-form-label width-7">Stat</div>
          <StatsPicker className="width-15" stats={selectedReducers} onChange={onStatsChange} />
        </div>
      </div>
    </>
  );
};

const registery = {
  [CalculateMode.separated]: SeparatedCalculationEditor,
  [CalculateMode.row]: RowCalculationEditor,
};

export const CalculateToRowTransformerEditor: React.FC<TransformerUIProps<CalculateToRowOptions>> = props => {
  const { options } = props;
  const mode = options.mode || CalculateMode.separated;

  const Editor = registery[mode];
  return (
    <>
      <BasicConfigRow {...props} />
      <Editor {...props} />
    </>
  );
};

function getFieldTypesFromDataFrame(data: DataFrame[]): { [key: string]: FieldType } {
  return data.reduce((types, frame) => {
    if (!frame || !Array.isArray(frame.fields)) {
      return types;
    }

    return frame.fields.reduce((types, field) => {
      const t = getFieldDisplayName(field, frame, data);

      types[t] = field.type;
      return types;
    }, types);
  }, {} as { [key: string]: FieldType });
}

export const calculateToRowTransformRegistryItem: TransformerRegistyItem<CalculateToRowOptions> = {
  id: DataTransformerID.calculateToRow,
  editor: CalculateToRowTransformerEditor,
  transformation: standardTransformers.calculateToRowTransformer,
  name: standardTransformers.calculateToRowTransformer.name,
  description: standardTransformers.calculateToRowTransformer.description,
};