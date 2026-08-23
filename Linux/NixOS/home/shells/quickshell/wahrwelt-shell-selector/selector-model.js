function profileFamily(option) {
  return option && option.family ? option.family : option.id;
}

function buildCards(options) {
  var cards = [];
  var indexes = {};

  for (var i = 0; i < options.length; i++) {
    var option = options[i];
    var family = profileFamily(option);
    var cardIndex = indexes[family];

    if (cardIndex === undefined) {
      cardIndex = cards.length;
      indexes[family] = cardIndex;
      cards.push({
        id: family,
        family: family,
        title: option.title,
        accent: option.accent,
        surface: option.surface,
        logo: option.logo,
        variants: []
      });
    }

    cards[cardIndex].variants.push(option);
  }

  return cards;
}

function activeFamily(options, activeId) {
  for (var i = 0; i < options.length; i++) {
    if (options[i].id === activeId) {
      return profileFamily(options[i]);
    }
  }
  return activeId;
}

function validEnd4Variant(profileId) {
  return profileId === "end4" || profileId === "end4-pc";
}

function end4Target(activeId, rememberedId) {
  if (validEnd4Variant(activeId)) {
    return activeId;
  }
  if (validEnd4Variant(rememberedId)) {
    return rememberedId;
  }
  return "end4";
}

function cardTarget(card, activeId, rememberedId) {
  if (card.family === "end4") {
    return end4Target(activeId, rememberedId);
  }
  if (card.variants && card.variants.length > 0) {
    return card.variants[0].id;
  }
  return card.id;
}
