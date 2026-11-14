"""Seed the isolated Docker ingredient preview. Uses only Python's standard library.

Run: python scripts/seed-ingredient-preview.py
Only localhost:8081 is supported; existing example slugs are left untouched.
"""
import json
import urllib.request
import urllib.error

BASE = 'http://localhost:8081/api/v1'
EMAIL = 'tester@cookira.local'
PASSWORD = 'CookiraTest!2026'
token = None
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def request(path, data=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    req = urllib.request.Request(BASE + path, data=None if data is None else json.dumps(data).encode(), headers=headers)
    try:
        with opener.open(req, timeout=15) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        raise RuntimeError(f'{path}: {error.code}: {error.read().decode()}') from error


status = request('/setup/status')
login = {'email': EMAIL, 'password': PASSWORD, 'tokens': True}
session = request('/setup', {**login, 'name': 'Ingredient Tester'}) if status['required'] else request('/auth/login', login)
token = session['tokens']['accessToken']


def entry(name, quantity=None, **fields):
    return {'name': name, **({'quantity': quantity} if quantity is not None else {}), **fields}


def group(name, *entries):
    return {'name': name, 'items': [], 'entries': entries}


examples = [
    {
        'title': 'Example: Scallion Chicken Rice Bowls', 'slug': 'example-scallion-chicken-rice-bowls',
        'tagline': 'Aliases, preparation differences, and compatible units merge in grocery view.',
        'servings': 4, 'prepMinutes': 15, 'cookMinutes': 25,
        'ingredients': [
            group('Chicken', entry('chicken thighs', 1, unit='lb', preparation='cut into strips'),
                  entry('scallions', 2, preparation='thinly sliced'), entry('olive oil', 1, unit='tbsp'),
                  entry('garlic', 2, unit='clove', preparation='minced')),
            group('Rice', entry('rice', 1, unit='cup'), entry('green onion', 3, preparation='finely diced'),
                  entry('olive oil', 3, unit='tsp'), entry('salt', notes='to taste'),
                  entry('cilantro', 0.25, unit='bunch', optional=True, preparation='chopped')),
        ],
        'steps': [{'text': 'Cook the rice according to package directions.'},
                  {'text': 'Cook chicken in olive oil with garlic until cooked through. Stir in the sliced scallions.'},
                  {'text': 'Fold diced green onion into the rice. Serve chicken over rice; add cilantro if desired.'}],
        'notes': 'Grocery view should show **5 green onion** and **2 tbsp olive oil**. Cilantro starts unchecked. Preparation remains visible in the recipe.',
    },
    {
        'title': 'Example: Roasted Onion and Lime Rice', 'slug': 'example-roasted-onion-lime-rice',
        'tagline': 'Ranges, sizes, optional ingredients, and unmeasured amounts remain explicit.',
        'servings': 4, 'prepMinutes': 10, 'cookMinutes': 30,
        'ingredients': [
            group('Roasted onions', entry('onion', 2, quantityMax=3, size='medium', preparation='cut into wedges'),
                  entry('olive oil', 2, unit='tbsp'), entry('garlic powder', 1, unit='tsp'),
                  entry('salt', notes='to taste')),
            group('Rice and finish', entry('rice', 200, unit='g'), entry('lime', 1, preparation='juiced'),
                  entry('onion', 1, size='small', preparation='finely diced'),
                  entry('cilantro', notes='as needed', optional=True)),
        ],
        'steps': [{'text': 'Toss onion wedges with olive oil and garlic powder. Roast at 425°F until tender and browned.'},
                  {'text': 'Cook the rice. Fold in the diced onion and lime juice, then top with roasted onions.'}],
        'notes': 'The **2–3 medium onions** stay separate from **1 small onion**. Unmeasured salt is retained; garlic powder has its own catalog identity.',
    },
    {
        'title': 'Example: Pantry Tomato Soup (Legacy Import)', 'slug': 'example-pantry-tomato-soup',
        'tagline': 'Original text survives; use Edit → Structure this ingredient to convert individual lines.',
        'servings': 4, 'prepMinutes': 10, 'cookMinutes': 20,
        'ingredients': [{'name': 'Soup', 'items': ['2 (14 oz) cans diced tomatoes', '1 medium onion, chopped',
                           '2 cups vegetable stock', '1 tbsp olive oil', 'salt and pepper to taste']}],
        'steps': [{'text': 'Soften the onion in olive oil. Add tomatoes and stock; simmer for 20 minutes.'},
                  {'text': 'Blend to your preferred texture and season to taste.'}],
        'notes': 'This intentionally uses the old text format. No quantities or ingredient identities are guessed during migration.',
    },
]

for recipe in examples:
    try:
        request('/recipes/' + recipe['slug'])
        print('Already exists:', recipe['title'])
        continue
    except RuntimeError as error:
        if ': 404:' not in str(error):
            raise
    saved = request('/recipes', {**recipe, 'tags': ['ingredient-preview', 'example'], 'status': 'published'})
    print('Created:', saved['title'])

print('Preview: http://localhost:8081')
print('Sign in:', EMAIL, '/', PASSWORD)
