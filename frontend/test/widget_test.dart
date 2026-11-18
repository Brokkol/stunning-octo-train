import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:frontend/main.dart';

void main() {
  testWidgets('экран входа показывает заголовок и кнопку Google', (
    WidgetTester tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: LoginScreen(title: 'Mini Imgur', onSignIn: () {}),
      ),
    );

    expect(find.text('Mini Imgur'), findsOneWidget);
    expect(find.text('Войти через Google'), findsOneWidget);
  });

  testWidgets('нажатие на кнопку входа вызывает onSignIn', (
    WidgetTester tester,
  ) async {
    var taps = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: LoginScreen(title: 'Mini Imgur', onSignIn: () => taps++),
      ),
    );

    await tester.tap(find.text('Войти через Google'));
    await tester.pump();

    expect(taps, 1);
  });

  testWidgets('переключатель темы меняет режим и запоминает выбор', (
    WidgetTester tester,
  ) async {
    SharedPreferences.setMockInitialValues({}); // пустое "хранилище" на время теста
    themeModeNotifier.value = ThemeMode.system;

    await tester.pumpWidget(
      const MaterialApp(home: Scaffold(body: ThemeModeSelector())),
    );

    await tester.tap(find.text('Тёмная'));
    await tester.pump();

    expect(themeModeNotifier.value, ThemeMode.dark);
    final prefs = await SharedPreferences.getInstance();
    expect(prefs.getString('theme_mode'), 'dark');
  });
}
