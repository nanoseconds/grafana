import { map } from 'rxjs/operators';
import isEmpty from 'lodash/isEmpty';
import values from 'lodash/values';

import { DataTransformerID } from './ids';
import { DataTransformerInfo } from '../../types/transformations';
import { fieldReducers, reduceField, ReducerID } from '../fieldReducer';
import { getFieldDisplayName } from '../../field/fieldState';
import { DataFrame, Field } from '../../types/dataFrame';
import { ArrayVector } from '../../vector';

export enum RowPlacement {
  top = 'top',
  bottom = 'bottom',
}

export enum CalculateMode {
  separated = 'separated',
  row = 'row',
}

export interface CalculateToRowOptions {
  reducers: { [key: string]: ReducerID[] };
  placement: RowPlacement;
  mode: CalculateMode;
}

export const rowPlacements = [
  { label: 'Top', value: RowPlacement.top },
  { label: 'Bottom', value: RowPlacement.bottom },
];

export const calculateModes = [
  { label: 'Separated calculate', value: CalculateMode.separated },
  { label: 'Calculate row', value: CalculateMode.row },
];

export const calculateToRowTransformer: DataTransformerInfo<CalculateToRowOptions> = {
  id: DataTransformerID.calculateToRow,
  name: 'Calculate To Row',
  description:
    'Append a new row by calculating each column to a signle value using a function like max, min, mean or last',
  defaultOptions: {
    reducers: {},
    placement: RowPlacement.bottom,
  },
  operator: options => source =>
    source.pipe(
      map(data => {
        if (values(options.reducers).every(item => isEmpty(item))) {
          return data;
        }

        return calculateToRow(data, options);
      })
    ),
};

export function calculateToRow(data: DataFrame[], options: CalculateToRowOptions): DataFrame[] {
  const reducerId = options.reducers;
  const placement = options.placement;
  const processed: DataFrame[] = [];

  const isPlaceAtTop = placement === RowPlacement.top;
  const prevStatRowTopIndex = data
    .flatMap(series => series.fields.flatMap(field => field.config.statValues || []))
    .filter(stateValue => stateValue.placement === RowPlacement.top)
    .reduce((topIndex, curr) => Math.max(curr.index.row, topIndex), -1);

  for (const series of data) {
    const fields: Field[] = [];
    for (const [col, field] of series.fields.entries()) {
      const fieldName = getFieldDisplayName(field, series, data);
      if (!fieldName) {
        continue;
      }
      const calculators = fieldReducers.list(reducerId[fieldName] || []);
      const reducers = calculators.map(c => c.id);

      const fieldValueArray = field.values.toArray();
      const statValues = field.config.statValues || [];
      const statValueIndexes = statValues.map(value => value.index.row);
      // Skip the previous calculated values
      const originalFieldValues = new ArrayVector(
        fieldValueArray.filter((_, index) => !statValueIndexes.includes(index))
      );

      const results = reduceField({
        field: {
          ...field,
          values: originalFieldValues,
        },
        reducers,
      });

      let value;
      let nextRowIndex = isPlaceAtTop ? prevStatRowTopIndex + 1 : field.values.length;
      let nextFieldValues = [...fieldValueArray];
      let nextStatValues = [...statValues];

      if (reducers.length > 0) {
        // Only the first stat takes effect
        const reducer = reducers[0];
        value = results[reducer];

        const prevRowIndex = statValues.reduce((prev, curr) => {
          if (curr.placement === placement) {
            return curr.index.row;
          }
          return prev;
        }, undefined as number | void);

        nextRowIndex = prevRowIndex ? prevRowIndex + 1 : nextRowIndex;

        nextStatValues = [
          ...statValues,
          {
            placement,
            id: reducer as ReducerID,
            index: {
              col,
              row: nextRowIndex,
            },
          },
        ];
      }

      nextFieldValues.splice(nextRowIndex, 0, value);
      const copy = {
        ...field,
        config: {
          ...field.config,
          statValues: [...nextStatValues],
        },
        values: new ArrayVector(nextFieldValues),
      };

      copy.state = undefined;

      fields.push(copy);
    }

    if (fields.length) {
      processed.push({
        ...series,
        fields,
        length: series.length + 1,
      });
    }
  }

  return processed;
}